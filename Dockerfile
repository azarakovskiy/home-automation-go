# Build stage
FROM golang:1.23-alpine AS builder
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/home-go ./cmd/home-go

# Runtime
FROM alpine:3.20
RUN apk --no-cache add ca-certificates tzdata
COPY --from=builder /out/home-go /usr/local/bin/home-go
ENTRYPOINT ["/usr/local/bin/home-go"]
