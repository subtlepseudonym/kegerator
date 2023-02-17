# syntax = docker/dockerfile:1.1-experimental
# Dockerfile.build
FROM golang:1.20-buster as base

ENV GNU_HOST=arm-linux-gnueabi
ENV CC=$GNU_HOST-gcc

RUN apt-get -y update
RUN apt-get --no-install-recommends install -y \
	gcc-$GNU_HOST \
	libc6-dev-armel-cross
RUN rm -rf /var/lib/apt/lists/*

WORKDIR /code
#COPY ./go.mod .
#RUN --mount=type=cache,target=/go/pkg/mod go mod download


FROM base as builder

WORKDIR /code

ENV CGO_ENABLED=1
ENV GOARCH=arm
ENV GOOS=linux
ENV BIN_DIR=/tmp/bin

COPY . .

RUN --mount=type=cache,target=/root/cache \
	#--mount=type=cache,target=/go/pkg/mod,ro \
	mkdir -p $BIN_DIR && \
	go build -mod vendor -o $BIN_DIR/kegerator -v *.go


FROM scratch
COPY --from=builder /tmp/bin /
COPY --from=subtlepseudonym/healthcheck:0.1.1 /healthcheck /healthcheck

EXPOSE 9220/tcp
HEALTHCHECK --interval=60s --timeout=2s --retries=3 --start-period=2s \
	CMD ["/healthcheck", "localhost:9220", "/ok"]

CMD ["/kegerator"]