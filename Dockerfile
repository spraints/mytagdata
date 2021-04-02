FROM golang:1.16.2

WORKDIR /src
COPY go.mod go.mod
COPY go.sum go.sum
RUN go mod download

COPY main.go main.go
COPY pkg pkg
RUN go build -o /usr/bin/receiver .

ENTRYPOINT ["/usr/bin/receiver"]
