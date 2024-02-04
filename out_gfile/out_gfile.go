package main

import "C"
import (
	"bytes"
	"encoding/json"
	"fluent-bit-go-plugins/utils"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"
	"unsafe"

	"github.com/fluent/fluent-bit-go/output"
)

const DefaultFilenameFormat = "$Tag-$Date.log"
const DefaultDateFormat = "%Y%m%d"
const DefaultLogFileNumber = 2
const MaxLogFileNumber = 10

// min seconds to delete file descriptor from cache
const LogFileLifeCycleMinSeconds = 3600

const OutFileLogFormat = "out_file"
const PlainLogFormat = "plain"
const DebugLog = false

type LogFile struct {
	File       *os.File
	OpenTime   time.Time
	ModifyTime time.Time
}

// go time format refererence: "2006-01-02 15:04:05.999999999 -0700 MST"
var (
	logFiles       = make(map[string]*LogFile, DefaultLogFileNumber)
	filenameFormat = DefaultFilenameFormat
	fileFormat     = OutFileLogFormat
	dateFormat     = DefaultDateFormat
	dateFormatVars = map[string]string{
		"%Y": "2006",
		"%m": "01",
		"%d": "02",
		"%H": "15",
		"%M": "04",
	}
	supportedLogFormats = []string{OutFileLogFormat, PlainLogFormat}
)

func init() {
	log.SetFlags(log.Ldate | log.Ltime | log.Lmicroseconds | log.Lshortfile)
}

func releaseLogFiles(checkLogFileNumber, checkLifeCycle bool) {
	if checkLogFileNumber && len(logFiles) <= MaxLogFileNumber {
		return
	}

	for path, logFile := range logFiles {
		needRelease := true
		if checkLifeCycle {

			modifyTime := logFile.ModifyTime
			if modifyTime.Add(time.Second * LogFileLifeCycleMinSeconds).After(time.Now()) {
				needRelease = false
			}

			if needRelease {
				file := logFile.File
				err := file.Sync()
				if err != nil {
					log.Println(err)
				}

				err = file.Close()
				if err != nil {
					log.Println(err)
				}

				if checkLifeCycle && DebugLog {
					openTime := logFile.OpenTime
					elapsedSeconds := modifyTime.Sub(openTime).Seconds()
					if DebugLog {
						log.Printf("delete log file: %s, used seconds: %.2f", path, elapsedSeconds)
					}
				}

				delete(logFiles, path)
			}
		}
	}
}

func getLogFile(path string) (*LogFile, error) {
	logFile, ok := logFiles[path]
	if ok {
		return logFile, nil
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		folder := filepath.Dir(path)
		if _, err := os.Stat(folder); os.IsNotExist(err) {
			if DebugLog {
				log.Printf("create folder: %s\n", folder)
			}
			err = os.MkdirAll(folder, os.ModePerm)
			if err != nil {
				return nil, err
			}
		} else if err != nil {
			return nil, err
		}
	} else if err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, os.ModePerm)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	logFile = &LogFile{
		File:       file,
		OpenTime:   now,
		ModifyTime: now,
	}

	logFiles[path] = logFile
	if DebugLog {
		log.Printf("open log file: %s", path)
	}

	releaseLogFiles(true, true)

	return logFile, nil
}

// func getTimestamp(ts interface{}) time.Time {
// 	var timestamp time.Time
// 	switch t := ts.(type) {
// 	case output.FLBTime:
// 		timestamp = ts.(output.FLBTime).Time
// 	case uint64:
// 		timestamp = time.Unix(int64(t), 0)
// 	default:
// 		timestamp = time.Now()
// 	}

// 	return timestamp
// }

func getLogFilename(tag string, timestamp time.Time) string {
	local := timestamp.Local()
	date := local.Format(dateFormat)
	filename := strings.ReplaceAll(filenameFormat, "$Tag", tag)
	filename = strings.ReplaceAll(filename, "$Date", date)

	return filename
}

func getString(v interface{}) string {
	var value string
	switch v.(type) {
	case []byte:
		value = fmt.Sprintf("%s", v)
	default:
		value = fmt.Sprintf("%v", v)
	}

	return value
}

func (l *LogFile) write(buf bytes.Buffer) (int, error) {
	file := l.File
	n, err := file.Write(buf.Bytes())
	if err != nil {
		return 0, err
	}

	l.ModifyTime = time.Now()
	if DebugLog {
		filename := file.Name()
		log.Printf("write to file: %s, bytes: %d", filename, n)
	}

	return n, nil
}

func (l *LogFile) writeOutFileRecord(tag string, timestamp time.Time, record map[string]interface{}) error {
	var buf bytes.Buffer
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	prefix := fmt.Sprintf("%s: [%d.%d, ", tag, timestamp.Unix(), timestamp.Nanosecond())
	buf.Write([]byte(prefix))
	buf.Write(data)
	buf.Write([]byte("]\n"))

	_, err = l.write(buf)
	if err != nil {
		return err
	}

	return nil
}

func (l *LogFile) writePlainRecord(tag string, timestamp time.Time, record map[string]interface{}) error {
	var buf bytes.Buffer
	data, err := json.Marshal(record)
	if err != nil {
		return err
	}
	buf.Write(data)
	buf.Write([]byte("\n"))

	_, err = l.write(buf)
	if err != nil {
		return err
	}

	return nil
}

func (l *LogFile) writeRecord(format, tag string, timestamp time.Time, record map[string]interface{}) error {
	var err error
	switch format {
	case OutFileLogFormat:
		err = l.writeOutFileRecord(tag, timestamp, record)
	case PlainLogFormat:
		err = l.writePlainRecord(tag, timestamp, record)
	default:
		err = fmt.Errorf("invalid log format: %s", format)
	}

	return err
}

//export FLBPluginRegister
func FLBPluginRegister(def unsafe.Pointer) int {
	// Gets called only once when the plugin.so is loaded
	log.Println("register gfile output plugin")

	return output.FLBPluginRegister(def, "gfile", "Go File Output Plugin")
}

//export FLBPluginInit
func FLBPluginInit(plugin unsafe.Pointer) int {
	// Gets called only once for each instance you have configured.
	configFile := output.FLBPluginConfigKey(plugin, "File")
	if configFile != "" {
		filenameFormat = configFile
	}

	configDate := output.FLBPluginConfigKey(plugin, "Date")
	if configDate != "" {
		for k, v := range dateFormatVars {
			configDate = strings.ReplaceAll(configDate, k, v)
		}

		dateFormat = configDate
	}

	configFormat := output.FLBPluginConfigKey(plugin, "Format")
	if configFormat != "" {
		fileFormat = configFormat
	}

	if !utils.Contains(supportedLogFormats, fileFormat) {
		log.Printf("unsupported log format: %s\n", fileFormat)
		return output.FLB_ERROR
	}

	return output.FLB_OK
}

//export FLBPluginFlushCtx
func FLBPluginFlushCtx(ctx, data unsafe.Pointer, length C.int, tag *C.char) int {
	// Gets called with a batch of records to be written to an instance.
	var ret int
	var ts interface{}
	var record map[interface{}]interface{}
	dec := output.NewDecoder(data, int(length))

	flbTag := C.GoString(tag)

	for {
		ret, ts, record = output.GetRecord(dec)
		if ret != 0 {
			break
		}

		newRecord, err := utils.ConvertRecord(record)
		if err != nil {
			log.Println(err)
			return output.FLB_ERROR
		}

		timestamp := utils.GetTimestamp(ts)
		filename := getLogFilename(flbTag, timestamp)
		logFile, err := getLogFile(filename)
		if err != nil {
			log.Println(err)
			return output.FLB_ERROR
		}

		err = logFile.writeRecord(fileFormat, flbTag, timestamp, newRecord)
		if err != nil {
			log.Println(err)
			return output.FLB_ERROR
		}
	}

	return output.FLB_OK
}

//export FLBPluginUnregister
func FLBPluginUnregister(def unsafe.Pointer) {

	if DebugLog {
		log.Println("unregister gfile plugin, close all files opened")
	}

	releaseLogFiles(false, false)
	if DebugLog {
		log.Println("all files have been closed")
	}

	output.FLBPluginUnregister(def)
}

//export FLBPluginExit
func FLBPluginExit() int {
	return output.FLB_OK
}

func main() {
}
