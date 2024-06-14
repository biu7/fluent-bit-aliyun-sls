# Use an appropriate base image
FROM golang:1.22-bullseye AS build

# Set the target architecture
ARG TARGETARCH

# Install dependencies
RUN apt-get install -y --no-install-recommends gcc libc6-dev

# Set the working directory
WORKDIR /app

# Copy the Go source code
COPY . .

# Build the Go application with CGO enabled
RUN GOARCH=amd64 GOOS=linux CGO_ENABLED=1 go build -buildmode=c-shared -o out_gsls.so ./out_gsls

FROM fluent/fluent-bit:3.0.7

COPY --from=build /app/out_gsls.so /fluent-bit/plugins/out_gsls.so