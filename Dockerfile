# syntax=docker/dockerfile:1

FROM golang:1.17-alpine
WORKDIR /app

# Download the modules
COPY go.mod ./
COPY go.sum ./
RUN go mod download

# Build the program
COPY cmd ./cmd
RUN go build -o /usr/bin/go-example-io -- ./cmd/go-example-io

# Run the program
CMD [ "/usr/bin/go-example-io", "--mode=combined", "--path=/tmp/go-example-io-file" ]
