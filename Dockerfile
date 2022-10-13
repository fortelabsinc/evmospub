FROM golang:latest AS build-env

WORKDIR /go/src/github.com/evmos/evmos

RUN apt-get update -y
RUN apt-get install git -y

COPY . .

RUN make build

FROM ubuntu:latest

RUN apt-get update -y
RUN apt-get install ca-certificates jq bc -y

WORKDIR /root

COPY --from=build-env /go/src/github.com/evmos/evmos/build/evmosd /usr/bin/evmosd
COPY --from=build-env /go/src/github.com/evmos/evmos/mud_init.sh .

EXPOSE 26656 26657 1317 9090

ENTRYPOINT ["./mud_init.sh"]