FROM golang:1.16-buster AS build
RUN apt update && apt install -y build-essential libavcodec-dev libavformat-dev libavresample-dev libswscale-dev

WORKDIR $GOPATH/src/github.com/josh23french/RTSPtoWSMP4f

COPY go.* .

# RUN go get -d -v ./...
RUN go mod download


COPY . .

RUN go install -v ./...

# FROM debian:buster-slim

# RUN apt update && apt install -y ca-certificates libavcodec58 libavformat58 libavresample4 libswscale5

# WORKDIR /root/

# COPY --from=build /go/bin/RTSPtoWSMP4f .
# COPY web/ ./web/
# COPY config.json .

EXPOSE 8083

ENTRYPOINT [ "RTSPtoWSMP4f" ]