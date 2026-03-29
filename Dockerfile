FROM golang:1.25-alpine AS builder

RUN apk add --no-cache git

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w -X github.com/ravensync/ravensync/internal/cli.Version=docker" \
    -o /bin/ravensync ./cmd/ravensync

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata
COPY --from=builder /bin/ravensync /usr/local/bin/ravensync

RUN mkdir -p /root/.ravensync
VOLUME ["/root/.ravensync"]

ENTRYPOINT ["ravensync"]
CMD ["serve"]
