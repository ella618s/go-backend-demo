# 階段一：編譯 Go 程式
FROM golang:1.27-alpine AS builder
WORKDIR /app

# 安裝基本編譯依賴與 CA 憑證
RUN apk add --no-cache ca-certificates git

COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s" -o main .

# 階段二：建立極輕量運行環境
FROM alpine:latest
WORKDIR /app

# 安裝時區資料庫與 CA 憑證
RUN apk add --no-cache tzdata ca-certificates

COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]