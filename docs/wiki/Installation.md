# 安装指南

## 系统要求

| 项目 | 要求 |
|------|------|
| 操作系统 | Windows 10+ / Linux / macOS |
| Go 版本 | 1.22+（仅源码编译需要） |
| 网络 | 稳定连接，建议配置代理 |
| 磁盘 | 数百 MB 空余（程序本体仅约 20MB） |

## 方式 1: 下载预编译版本（推荐）

从 [Releases](https://github.com/Feng4/movie_data_capture_go/releases) 下载对应平台的压缩包：

| 系统 | 文件名 |
|------|--------|
| Windows x64 | `mdc-windows-amd64.zip` |
| Linux x64 | `mdc-linux-amd64.tar.gz` |
| macOS Intel | `mdc-darwin-amd64.tar.gz` |
| macOS Apple Silicon | `mdc-darwin-arm64.tar.gz` |

解压后：

```bash
# Linux / macOS 赋予执行权限
chmod +x mdc

# 验证
./mdc -version
# 输出: Movie Data Capture Go Version 1.1.0 (含 Go 版本与平台信息)
```

> **注意**: 压缩包内的 `Img/` 目录存放水印角标素材（中字、流出、4K 等），程序运行时需要读取，请保持目录结构完整，不要只拷贝可执行文件。

## 方式 2: 从源码编译

```bash
# 1. 克隆仓库
git clone https://github.com/Feng4/movie_data_capture_go.git
cd movie_data_capture_go

# 2. 下载依赖
go mod download

# 3. 编译当前平台
go build -o mdc main.go

# Windows PowerShell 下
go build -o mdc.exe main.go
```

### 交叉编译其他平台

```bash
# Linux x64
GOOS=linux   GOARCH=amd64 go build -o mdc-linux main.go

# Windows x64
GOOS=windows GOARCH=amd64 go build -o mdc.exe main.go

# macOS Apple Silicon
GOOS=darwin  GOARCH=arm64 go build -o mdc-mac-arm main.go
```

> Windows PowerShell 交叉编译时若报 `go: unsupported GOOS`，请先确认 Go 安装完整；PowerShell 中设置环境变量用 `$env:GOOS="linux"` 形式。

## 方式 3: Docker

仓库自带 `Dockerfile`（多阶段构建，最终镜像基于 alpine）与 `docker-compose.yml`：

```bash
# 使用 docker compose 一键启动
docker compose up movie-data-capture
```

compose 默认挂载：

| 宿主机目录 | 容器路径 | 用途 |
|-----------|---------|------|
| `./config.yaml` | `/app/config.yaml` (只读) | 配置文件 |
| `./movies` | `/app/movies` (只读) | 待处理电影 |
| `./JAV_output` | `/app/JAV_output` | 成功输出 |
| `./failed` | `/app/failed` | 失败输出 |
| `./logs` | `/app/logs` | 日志目录 |

默认执行命令为 `-path /app/movies -logdir /app/logs`。

**代理注意**：容器内 `127.0.0.1` 指向容器自身。宿主机代理需改填宿主机局域网 IP（如 `192.168.1.100:10808`），或取消 compose 文件中 `network_mode: "host"` 的注释使用宿主网络。

## 安装验证清单

```bash
./mdc -version                 # 版本信息正常输出
./mdc                          # 自动生成默认 config.yaml
./mdc -search "SSIS-001"       # 试搜一个番号，验证网络/代理连通
```

搜索能返回影片信息，即安装与网络配置全部就绪。

## 下一步

- [快速开始](Quick-Start.md): 处理第一个文件
- [配置详解](Configuration.md): 定制 config.yaml
