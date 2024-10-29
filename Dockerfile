# 构建阶段
FROM golang:1.22-alpine AS builder

ARG VERSION
ARG COMMIT

# 安装 git 和 ca-certificates (需要 git 来获取私有依赖，如果有的话)
RUN apk add --no-cache git ca-certificates tzdata && update-ca-certificates

# 设置工作目录
WORKDIR /app

# 复制 go mod 和 sum 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux go build -ldflags="-w -s -X main.Version=${VERSION} -X main.Commit=${COMMIT}" -a -installsuffix cgo -o gpgenie ./cmd/gpgenie

# 最终阶段
FROM scratch

# 从 builder 阶段复制 ca-certificates
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/
COPY --from=builder /usr/share/zoneinfo /usr/share/zoneinfo
ENV TZ=Asia/Shanghai

WORKDIR /root/

# 从构建器阶段复制二进制文件
COPY --from=builder /app/gpgenie .
COPY --from=builder /app/config/config.sqlite.json ./config/config.json

# 运行
CMD ["./gpgenie"]
