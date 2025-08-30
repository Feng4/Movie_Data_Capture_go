# 多阶段构建 Dockerfile
FROM golang:1.21-alpine AS builder

# 安装必要的工具
RUN apk add --no-cache git ca-certificates tzdata

# 设置工作目录
WORKDIR /app

# 复制 go mod 文件
COPY go.mod go.sum ./

# 下载依赖
RUN go mod download

# 复制源代码
COPY . .

# 构建应用
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
    -ldflags="-s -w" \
    -o mdc main.go

# 最终运行镜像
FROM alpine:latest

# 安装必要的运行时依赖
RUN apk --no-cache add ca-certificates tzdata

# 创建非root用户
RUN adduser -D -s /bin/sh mdc

# 设置工作目录
WORKDIR /app

# 从builder阶段复制二进制文件
COPY --from=builder /app/mdc /app/
COPY --from=builder /app/config.yaml /app/
COPY --from=builder /app/Img /app/Img/

# 创建必要的目录
RUN mkdir -p /app/JAV_output /app/failed /app/logs

# 设置权限
RUN chown -R mdc:mdc /app

# 切换到非root用户
USER mdc

# 暴露端口（如果需要）
# EXPOSE 8080

# 设置环境变量
ENV TZ=Asia/Shanghai

# 健康检查
HEALTHCHECK --interval=30s --timeout=10s --start-period=5s --retries=3 \
    CMD /app/mdc -version || exit 1

# 启动命令
ENTRYPOINT ["/app/mdc"]