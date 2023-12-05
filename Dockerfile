FROM golang:1.21-bookworm AS builder

RUN go run github.com/playwright-community/playwright-go/cmd/playwright@latest install chromium --with-deps

WORKDIR /go/src/github.com/at-wat/twfeed

COPY go.mod go.sum /go/src/github.com/at-wat/twfeed/
RUN go mod download

COPY . /go/src/github.com/at-wat/twfeed
RUN go build . && rm -rf $(go env GOCACHE)

WORKDIR /
ENTRYPOINT ["/go/src/github.com/at-wat/twfeed/twfeed"]
CMD []
