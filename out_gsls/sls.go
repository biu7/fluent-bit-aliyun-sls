package main

import (
	"errors"
	"fluent-bit-go-plugins/utils"
	"fmt"
	"log"
	"os"

	sls "github.com/aliyun/aliyun-log-go-sdk"
	"google.golang.org/protobuf/proto"
	"gopkg.in/yaml.v3"
)

const (
	DockerEnvLogStoreKey = "FLUENTD_LOGSTORE"

	SlsLogContentMaxSizeBytes = 1024 * 1024
)

type SLSConfig struct {
	EnvKey          string   `yaml:"env_key"`
	AccessKeyID     string   `yaml:"access_key_id"`
	AccessKeySecret string   `yaml:"access_key_secret"`
	Endpoint        string   `yaml:"endpoint"`
	Project         string   `yaml:"project"`
	Stores          []string `yaml:"stores"`
}

type SLS struct {
	conf      *SLSConfig
	client    sls.ClientInterface
	logStores map[string]struct{}
}

func NewSLS(path string) (*SLS, error) {
	configBytes, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read aliyun sls config file failed, err: %w", err)
	}
	var configs SLSConfig
	err = yaml.Unmarshal(configBytes, &configs)
	if err != nil {
		return nil, fmt.Errorf("unmarshal aliyun sls config file failed, err: %w", err)
	}
	if configs.AccessKeyID == "" || configs.AccessKeySecret == "" || configs.Endpoint == "" {
		log.Println("empty aliyun config", path, string(configBytes))
		return nil, errors.New("empty aliyun config")
	}

	provider := sls.NewStaticCredentialsProvider(configs.AccessKeyID, configs.AccessKeySecret, "")
	client := sls.CreateNormalInterfaceV2(configs.Endpoint, provider)
	var ok bool
	ok, err = client.CheckProjectExist(configs.Project)
	if err != nil {
		return nil, fmt.Errorf("check project exist failed, project: %s, err: %w", configs.Project, err)
	}
	if !ok {
		return nil, fmt.Errorf("sls project %s does not exist, please create it first", configs.Project)
	}

	var logStores = make(map[string]struct{})
	for _, store := range configs.Stores {
		ok, err = client.CheckLogstoreExist(configs.Project, store)
		if err != nil {
			return nil, fmt.Errorf("check logstore exist failed, project: %s, logstore: %s, err: %w", configs.Project, store, err)
		}
		if !ok {
			return nil, fmt.Errorf("sls project %s logstore %s does not exist yet, please create it first", configs.Project, store)
		}
		logStores[store] = struct{}{}
	}
	return &SLS{
		conf:      &configs,
		client:    client,
		logStores: logStores,
	}, nil
}

func (s *SLS) PutLogs(tag, logStore string, logs []*sls.Log) error {
	logGroup := &sls.LogGroup{
		Topic:  proto.String(tag),
		Source: proto.String(GoSlsPluginSource),
		Logs:   logs,
	}

	err := s.client.PutLogs(s.conf.Project, logStore, logGroup)
	if err != nil {
		return err
	}
	return nil
}

func (s *SLS) Close() {
	if s != nil && s.client != nil {
		_ = s.client.Close()
	}
}

type Record struct {
	Ret       int
	Timestamp any
	Content   map[any]any
}

func (s *SLS) PutRecords(flbTag string, records []Record) {
	if len(records) == 0 {
		return
	}
	var logs = make(map[string][]*sls.Log)
	for _, record := range records {
		if record.Content == nil {
			continue
		}
		// 不包含 log store key，跳过
		storeVal, ok := record.Content[s.conf.EnvKey]
		if !ok {
			continue
		}
		// 不在配置的 log store 列表中，跳过
		store := utils.GetString(storeVal)
		if _, ok = s.logStores[store]; !ok {
			continue
		}

		timestamp := utils.GetTimestamp(record.Timestamp)
		var contents = []*sls.LogContent{
			{
				Key:   proto.String("_time_"),
				Value: proto.String(timestamp.Format("2006-01-02T15:04:05.999999999Z")),
			},
		}
		for k, v := range record.Content {
			key := utils.GetString(k)
			value := utils.GetString(v)
			content := &sls.LogContent{
				Key:   &key,
				Value: &value,
			}
			contentSize := content.Size()
			if contentSize > SlsLogContentMaxSizeBytes {
				content.Value = proto.String("value too large, discard")
			}
			contents = append(contents, content)
		}
		logs[store] = append(logs[store], &sls.Log{
			Contents: contents,
			Time:     proto.Uint32(uint32(timestamp.Unix())),
			TimeNs:   proto.Uint32(uint32(timestamp.Nanosecond())),
		})
	}

	if len(logs) == 0 {
		return
	}
	for store, batchLogs := range logs {
		if err := slsClient.PutLogs(flbTag, store, batchLogs); err != nil {
			log.Printf("put logs to sls failed, tag: %s, store: %s, err: %v\n", flbTag, store, err)
		}
	}
}
