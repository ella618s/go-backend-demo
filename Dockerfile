# 打包 Go 應用程式

# 階段一：編譯 Go 程式
FROM golang:1.27-alpine AS builder
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main .

# 階段二：建立極輕量運行環境
FROM alpine:latest
WORKDIR /app

# 👈 新增這行：安裝時區資料庫，解決 Asia/Taipei 找不到的問題
RUN apk add --no-cache tzdata

COPY --from=builder /app/main .
EXPOSE 8080
CMD ["./main"]