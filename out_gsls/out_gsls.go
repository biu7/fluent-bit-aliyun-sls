package main

import "C"
import (
	"log"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

const GoSlsPluginSource = "fluent-bit-gsls-plugin"

const DebugLog = false

var (
	slsClient *SLS
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
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
	configPath := output.FLBPluginConfigKey(plugin, "sls_config_path")
	if configPath == "" {
		log.Println("empty aliyun sls ak found, please set Sls_Ak field")
		return output.FLB_ERROR
	}
	var err error
	slsClient, err = NewSLS(configPath)
	if err != nil {
		log.Println(err.Error())
		return output.FLB_ERROR
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

	var records []Record
	for {
		ret, ts, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}
		records = append(records, Record{
			Ret:       ret,
			Timestamp: ts,
			Content:   record,
		})
	}

	slsClient.PutRecords(flbTag, records)
	return output.FLB_OK
}

//export FLBPluginUnregister
func FLBPluginUnregister(def unsafe.Pointer) {
	slsClient.Close()
	output.FLBPluginUnregister(def)
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {}
