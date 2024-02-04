package utils

import (
	"fmt"
	"time"

	"github.com/fluent/fluent-bit-go/output"
)

func GetString(v interface{}) string {
	var value string
	switch v.(type) {
	case []byte:
		value = fmt.Sprintf("%s", v)
	default:
		value = fmt.Sprintf("%v", v)
	}

	return value
}

func GetTimestamp(ts interface{}) time.Time {
	var timestamp time.Time
	switch t := ts.(type) {
	case output.FLBTime:
		timestamp = ts.(output.FLBTime).Time
	case uint64:
		timestamp = time.Unix(int64(t), 0)
	default:
		timestamp = time.Now()
	}

	return timestamp
}

func ConvertRecord(record map[interface{}]interface{}) (map[string]interface{}, error) {
	newRecord := make(map[string]interface{})
	for k, v := range record {
		key := GetString(k)
		newRecord[key] = v
	}

	return newRecord, nil
}

func Contains[T comparable](list []T, target T) bool {
	for _, l := range list {
		if l == target {
			return true
		}
	}

	return false
}
