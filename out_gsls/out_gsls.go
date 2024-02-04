package main

import "C"
import (
	"fluent-bit-go-plugins/utils"
	"log"
	"os"
	"unsafe"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"github.com/fluent/fluent-bit-go/output"
	"github.com/gogo/protobuf/proto"
)

const SlsLogContentMaxSizeBytes = 1024 * 1024
const SlsLogGroupMaxSizeBytes = 9 * 1024 * 1024
const GoSlsPluginSource = "fluent-bit-gsls-plugin"

const DebugLog = false

var (
	slsAkId     = os.Getenv("SLS_AK_ID")
	slsAkSecret = os.Getenv("SLS_AK_SECRET")
	slsEndpoint = os.Getenv("SLS_ENDPOINT")
	slsProject  = os.Getenv("SLS_PROJECT")
	slsLogStore = os.Getenv("SLS_LOGSTORE")
	slsClient   sls.ClientInterface
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func commitLogs(tag, source string, logs []*sls.Log) error {
	logGroup := &sls.LogGroup{
		Topic:  proto.String(tag),
		Source: proto.String(GoSlsPluginSource),
		Logs:   logs,
	}

	err := slsClient.PutLogs(slsProject, slsLogStore, logGroup)
	if err != nil {
		return err
	}

	if DebugLog {
		log.Printf("put logs to sls, logs: %d,  bytes: %d\n", len(logGroup.Logs), logGroup.Size())
	}

	return nil
}

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	// Gets called only once when the plugin.so is loaded
	log.Println("register gsls output plugin")

	return output.FLBPluginRegister(def, "gsls", "Go Sls Output Plugin")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	// Gets called only once for each instance you have configured.
	configAk := output.FLBPluginConfigKey(plugin, "Sls_Ak_Id")
	if configAk != "" {
		slsAkId = configAk
	}

	if slsAkId == "" {
		log.Println("empty aliyun sls ak found, please set Sls_Ak field")
		return output.FLB_ERROR
	}

	configSecret := output.FLBPluginConfigKey(plugin, "Sls_Ak_Secret")
	if configSecret != "" {
		slsAkSecret = configSecret
	}

	if slsAkSecret == "" {
		log.Println("empty aliyun sls secret found, please set Sls_Secret field")
		return output.FLB_ERROR
	}

	configEndpoint := output.FLBPluginConfigKey(plugin, "Sls_Endpoint")
	if configEndpoint != "" {
		slsEndpoint = configEndpoint

	}

	if slsEndpoint == "" {
		log.Println("empty aliyun sls endpoint found, please set Sls_Endpoint field")
		return output.FLB_ERROR
	}

	configProject := output.FLBPluginConfigKey(plugin, "Sls_Project")
	if configProject != "" {
		slsProject = configProject
	}

	if slsProject == "" {
		log.Println("empty aliyun sls project found, please set Sls_Project field")
		return output.FLB_ERROR
	}

	configLogStore := output.FLBPluginConfigKey(plugin, "Sls_LogStore")
	if configLogStore != "" {
		slsLogStore = configLogStore
	}

	if slsLogStore == "" {
		log.Println("empty aliyun sls logstore found, please set Sls_LogStore field")
		return output.FLB_ERROR
	}

	provider := sls.NewStaticCredentialsProvider(slsAkId, slsAkSecret, "")
	slsClient = sls.CreateNormalInterfaceV2(slsEndpoint, provider)

	ok, err := slsClient.CheckProjectExist(slsProject)
	if err != nil {
		log.Println(err)
		return output.FLB_ERROR
	}

	if !ok {
		log.Printf("sls project %s does not exist, please create it first\n", slsProject)
		return output.FLB_ERROR
	}

	ok, err = slsClient.CheckLogstoreExist(slsProject, slsLogStore)
	if err != nil {
		log.Println(err)
		return output.FLB_ERROR
	}

	if !ok {
		log.Printf("sls project %s logstore %s does not exist yet, please create it first\n", slsProject, slsLogStore)
	}

	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	// Gets called with a batch of records to be written to an instance.
	dec := output.NewDecoder(data, int(length))
	flbTag := C.GoString(tag)

	var ret int
	var ts interface{}
	var record map[interface{}]interface{}

	logs := []*sls.Log{}
	for {
		ret, ts, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}

		timestamp := utils.GetTimestamp(ts)
		contents := []*sls.LogContent{}
		for k, v := range record {
			key := utils.GetString(k)
			value := utils.GetString(v)

			content := &sls.LogContent{
				Key:   &key,
				Value: &value,
			}

			contentSize := content.Size()
			if contentSize > SlsLogContentMaxSizeBytes {
				log.Printf("sls log content exceed limit, key: %v, size: %d, limit: %d\n", k,
					contentSize, SlsLogContentMaxSizeBytes)

				return output.FLB_ERROR
			}

			contents = append(contents, content)
		}

		log := &sls.Log{
			Contents: contents,
			Time:     proto.Uint32(uint32(timestamp.Unix())),
			TimeNs:   proto.Uint32(uint32(timestamp.Nanosecond())),
		}

		logs = append(logs, log)
	}

	batchLogs := make([]*sls.Log, 0)
	batchSize := 0
	for _, slsLog := range logs {
		logSize := slsLog.Size()
		if batchSize+logSize > SlsLogGroupMaxSizeBytes {
			if err := commitLogs(flbTag, GoSlsPluginSource, batchLogs); err != nil {
				log.Panicln(err)

				return output.FLB_ERROR
			}

			batchLogs = make([]*sls.Log, 0)
			batchSize = 0
		} else {
			batchLogs = append(batchLogs, slsLog)
			batchSize += logSize
		}
	}

	if len(batchLogs) == 0 {
		return output.FLB_OK
	}

	if err := commitLogs(flbTag, GoSlsPluginSource, batchLogs); err != nil {
		log.Panicln(err)

		return output.FLB_ERROR
	}

	return output.FLB_OK
}

//export FLBPluginUnregister
func FLBPluginUnregister(def unsafe.Pointer) {
	err := slsClient.Close()
	if err != nil {
		log.Println(err)
	}

	output.FLBPluginUnregister(def)
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}
