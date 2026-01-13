FROM golang:alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /gox ./cmd/gox

FROM golang:alpine
RUN apk add --no-cache git
COPY --from=builder /gox /usr/local/bin/gox
RUN gox zig update
WORKDIR /src
ENTRYPOINT ["gox"]