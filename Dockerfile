FROM golang:1.12-alpine AS build

WORKDIR /go/src/github.com/jordan/monitor

COPY vendor vendor
COPY *.go ./
RUN go build -o build/monitor .

FROM alpine:3.9
RUN touch /tmp/access.log
COPY --from=build /go/src/github.com/jordan/monitor/build/monitor /usr/bin/
ENTRYPOINT ["/usr/bin/monitor"]

