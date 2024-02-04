# fluent-bit-go-plugins

Fluent Bit go plugins to enhance [fluent-bit](https://docs.fluentbit.io/manual/development/golang-output-plugins).

## Plugins

### gfile

gfile plugin support writing log record to a file whose name can be determined by record tag and timestamp. 

```
[OUTPUT]
    name    gfile
    match   cpu.local
    file    /logs/$Tag-$Date.log
    format  out_file
    date    %Y%m%d%H
```

date field in gfile output config support the following format:

- %Y year 2006
- %m month 01-12
- %d day 01-31
- %H hour 01-23
- %M minute 01-60

### gsls

gsls plugin support writing log record aliyun sls service.

```
[OUTPUT]
    name            gsls
    match           cpu.local
    sls_ak_id       your_sls_ak_id
    sls_ak_secret   your_sls_ak_secret
    sls_endpoint    your_sls_end_point
    sls_project     your_sls_project
    sls_logstore    your_sls_logstore
```

you may set sls config globally by environment variables, refer environment section in [docker-compose](./docker-compose.yml.example) file.

## Get Started

### Local

Prerequisites:

- Docker and docker-compose
  Docker Engine 23.0 and Docker Desktop 4.19 or above are needed since we need buildx
- OS Linux/Mac
  Windows is not tested yet
- CPU arch linux/amd64,linux/arm64

```
# steps
1. build docker container
docker-compose build
2. change fluent-bit.conf according to your need
3. start docker container
docker-compose up
```

### Docker Image

I have set github action to build container image supported linux/amd64 and linux/arm64, you may use it directly.

Dockerhub image: willmao/fluent-bit:[tag]

Alibaba ACR image for user in China: registry.cn-hangzhou.aliyuncs.com/willmao/fluent-bit:[tag]

Docker image tag format: v(fluent-bit-version)-(release-version), for example v2.2.2-0.0.1.