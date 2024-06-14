# fluent-bit-go-plugins

采集 docker 日志并投递到阿里云 sls 日志库

sls_config.yaml
```
env_key: "FLUENTD_LOGSTORE" # 配置在需要被采集的容器 env 中，env 值为需要投递到的日志库
access_key_id: "access_key_id"
access_key_secret: "access_key_secret"
endpoint: "cn-hongkong.log.aliyuncs.com"
project: "xxxxx-prod" # 需要投递到的 sls project
stores: # 可用的 logstore 列表
    - "xxx-server-logs"
    - "xxx-console-logs"
```

docker-compose.yml
```
services:
  app:
    container_name: app
    image: app:v1
    ports:
      - "8080:8080"
    environment:
      FLUENTD_LOGSTORE: "to_logstore"
    logging:
      driver: fluentd
      options:
        fluentd-address: localhost:24224
        tag: app
```
