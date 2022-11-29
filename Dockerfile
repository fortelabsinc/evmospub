FROM golang:latest AS build-env

WORKDIR /go/src/github.com/evmos/evmos

RUN apt-get update -y
RUN apt-get install git -y

COPY . .

RUN make install

FROM ubuntu:latest

RUN apt-get update -y
RUN apt-get install ca-certificates jq bc -y

WORKDIR /root

COPY --from=build-env /go/bin/evmosd /usr/bin/evmosd
COPY --from=build-env /go/src/github.com/evmos/evmos/evmos_init.sh .

EXPOSE 26656 26657 1317 9090 8545 8546

ENTRYPOINT ["./evmos_init.sh"]
