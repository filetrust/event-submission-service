FROM golang:alpine AS builder
WORKDIR /go/src/github.com/filetrust/event-submission-service
COPY . .
RUN cd cmd \
    && env GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o  event-submission-service .

RUN apk --no-cache add ca-certificates

FROM scratch
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /go/src/github.com/filetrust/event-submission-service/cmd/event-submission-service /bin/event-submission-service

ENTRYPOINT ["/bin/event-submission-service"]
