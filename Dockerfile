# syntax=docker/dockerfile:1

FROM golang:1.17-alpine as builder
WORKDIR /app

# Download the modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Build the program
COPY cmd ./cmd
RUN go build -o /opt/go-example-io -- ./cmd/go-example-io

# Run the program
FROM ubuntu:hirsute as runner
COPY --from=builder /opt/go-example-io /usr/bin/go-example-io
ENTRYPOINT [ "/usr/bin/go-example-io" ]
