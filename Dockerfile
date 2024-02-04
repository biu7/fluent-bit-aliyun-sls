# syntax=docker/dockerfile:1

ARG GO_VERSION="1.20.12"
ARG FLUENT_BIT_VERSION="2.2.2"
ARG FLUENT_BIT_GO_PLUGINS_VERSION="0.0.1"

FROM --platform=$BUILDPLATFORM crazymax/goxx:${GO_VERSION} AS base
ENV GO111MODULE=auto
ENV CGO_ENABLED=1

FROM base AS build
ARG TARGETPLATFORM

RUN --mount=type=cache,sharing=private,target=/var/cache/apt \
  --mount=type=cache,sharing=private,target=/var/lib/apt/lists \
  goxx-apt-get install -y binutils gcc g++ pkg-config

WORKDIR /go/release
ADD . /go/release/src
WORKDIR /go/release/src

RUN --mount=type=bind,source=. \
  --mount=type=cache,target=/root/.cache \
  --mount=type=cache,target=/go/pkg/mod \
  goxx-go build -buildmode=c-shared -o /out/plugins/out_gfile.so out_gfile/out_gfile.go \
  && goxx-go build -buildmode=c-shared -o /out/plugins/out_gsls.so out_gsls/out_gsls.go


FROM fluent/fluent-bit:${FLUENT_BIT_VERSION}

COPY --from=build /out/plugins /fluent-bit/plugins