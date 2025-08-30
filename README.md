# Movie Data Capture Go Version

[![Go Version](https://img.shields.io/badge/Go-1.21+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](#installation)

Movie Data Capture Go 是一个用 Go 语言编写的电影元数据自动抓取和整理工具。它能够从多个网站抓取电影信息，生成 NFO 文件，下载封面图片，并自动整理电影文件到指定目录结构。

## 🎯 主要功能

### 核心功能
- **多站点数据抓取**: 支持 JavaBus、FANZA、FC2、JavDB、X-City 等多个数据源
- **智能番号解析**: 自动从文件名中提取番号信息
- **分片文件处理**: 自动识别和处理多部分电影文件（CD1/CD2、Part1/Part2等）
- **NFO 文件生成**: 生成符合 Kodi/Jellyfin 标准的 NFO 元数据文件
- **图片下载**: 自动下载封面图、海报、剧照等
- **文件整理**: 按照自定义规则整理文件目录结构

### 高级功能
- **人脸检测**: 自动裁剪演员头像
- **水印处理**: 为图片添加自定义水印
- **翻译支持**: 自动翻译标题和简介为中文
- **代理支持**: 支持 HTTP/SOCKS5 代理
- **多线程处理**: 支持并发处理提高效率
- **失败重试**: 智能重试机制处理网络异常

## 🚀 快速开始

### 系统要求

- Go 1.21 或更高版本
- 稳定的网络连接（建议使用代理）
- Windows、Linux 或 macOS 操作系统

### 安装方法

#### 方法 1: 从源码编译

```bash
# 克隆仓库
git clone https://github.com/Feng4/movie_data_capture_go.git
cd movie-data-capture-go

# 编译程序
go mod download
go build -o mdc main.go

# 运行程序
./mdc
```

#### 方法 2: 直接下载二进制文件

从 [Releases](https://github.com/Feng4/movie_data_capture_go/releases) 页面下载适合你系统的预编译二进制文件。

### 基本使用

1. **配置设置**: 编辑 `config.yaml` 文件
2. **处理单个文件**:
   ```bash
   ./mdc -file "SSIS-001.mp4"
   ```
3. **批量处理目录**:
   ```bash
   ./mdc -path "/path/to/movies"
   ```
4. **搜索番号信息**:
   ```bash
   ./mdc -search "SSIS-001"
   ```

## 📋 命令行参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-config` | 配置文件路径 | `-config config.yaml` |
| `-file` | 单个文件处理 | `-file "movie.mp4"` |
| `-path` | 处理目录路径 | `-path "/movies"` |
| `-number` | 自定义番号 | `-number "SSIS-001"` |
| `-mode` | 运行模式 (1=抓取, 2=整理, 3=分析) | `-mode 1` |
| `-search` | 搜索番号 | `-search "SSIS-001"` |
| `-source` | 指定数据源 | `-source "javbus"` |
| `-url` | 指定URL | `-url "https://..."` |
| `-debug` | 启用调试模式 | `-debug` |
| `-version` | 显示版本信息 | `-version` |
| `-logdir` | 日志目录 | `-logdir "./logs"` |

## ⚙️ 配置说明

### 基础配置 (`config.yaml`)

```yaml
common:
  main_mode: 1                          # 1=抓取, 2=整理, 3=分析
  source_folder: "./"                   # 源文件夹
  success_output_folder: "JAV_output"   # 成功输出文件夹
  failed_output_folder: "failed"       # 失败输出文件夹
  link_mode: 0                         # 0=移动, 1=软链接, 2=硬链接

proxy:
  switch: true                         # 启用代理
  proxy: "127.0.0.1:10808"            # 代理地址
  type: "socks5"                       # 代理类型

name_rule:
  location_rule: "actor + '/' + number"        # 文件夹规则
  naming_rule: "number + '-' + title"          # 命名规则
  max_title_len: 50                            # 最大标题长度
```

### 支持的网站优先级

```yaml
priority:
  website: "javbus,fanza,fc2,fc2club,javdb,xcity,mgstage"
```

## 🎨 输出结构

处理完成后，文件将按以下结构整理：

```
JAV_output/
├── 演员名/
│   ├── SSIS-001-电影标题/
│   │   ├── SSIS-001.mp4           # 电影文件
│   │   ├── SSIS-001.nfo           # 元数据文件
│   │   ├── poster.jpg             # 海报
│   │   ├── fanart.jpg             # 封面
│   │   ├── thumb.jpg              # 缩略图
│   │   └── extrafanart/           # 额外图片
│   │       ├── extrafanart-1.jpg
│   │       └── extrafanart-2.jpg
```

## 🔧 分片文件处理

程序能够智能识别和处理分片文件：

- **CD 格式**: `SSIS-001-cd1.mp4`, `SSIS-001-cd2.mp4`
- **Part 格式**: `SSIS-001_part_1.mp4`, `SSIS-001_part_2.mp4`
- **数字格式**: `SSIS-001_1.mp4`, `SSIS-001_2.mp4`
- **字母格式**: `SSIS-001-A.mp4`, `SSIS-001-B.mp4`
- **Disc 格式**: `SSIS-001-disc1.mkv`, `SSIS-001-disc2.mkv`

## 🌐 支持的数据源

| 网站 | 类型 | 说明 |
|------|------|------|
| JavBus | 综合 | 主要数据源 |
| FANZA | 官方 | 高质量数据 |
| JavDB | 综合 | 备用数据源 |
| FC2 | 素人 | FC2 作品专用 |
| X-City | 无码 | 无码作品 |
| MGStage | 综合 | 备用数据源 |

## 📊 项目结构

```
movie-data-capture/
├── cmd/                    # 命令行入口
├── internal/              # 内部包
│   ├── config/           # 配置管理
│   ├── core/             # 核心处理逻辑
│   └── scraper/          # 数据抓取器
├── pkg/                   # 公共包
│   ├── downloader/       # 下载器
│   ├── fragment/         # 分片文件处理
│   ├── httpclient/       # HTTP 客户端
│   ├── logger/           # 日志系统
│   ├── nfo/              # NFO 生成器
│   ├── parser/           # 番号解析器
│   └── utils/            # 工具函数
├── config.yaml           # 配置文件
└── main.go               # 程序入口
```

## 🧪 测试工具

项目包含多个测试工具用于调试和验证功能：

- `debug_fragment_processing.go` - 分片处理调试
- `test_fragment_detection.go` - 分片检测测试
- `test_fragment_integration.go` - 集成测试
- `test_main_with_fragments.go` - 主程序测试

## 🤝 贡献指南

我们欢迎各种形式的贡献：

1. **报告问题**: 在 [Issues](https://github.com/Feng4/movie_data_capture_go/issues) 中报告 Bug
2. **功能建议**: 提出新功能想法
3. **代码贡献**: 提交 Pull Request
4. **文档改进**: 完善文档和示例

### 开发环境设置

```bash
# 克隆仓库
git clone https://github.com/Feng4/movie_data_capture_go.git
cd movie-data-capture-go

# 安装依赖
go mod download

# 运行测试
go test ./...

# 编译程序
go build -o mdc main.go
```

## 📝 更新日志

### v1.0.0 (2025-08-30)
- ✨ 首个正式版本发布
- 🚀 支持多站点数据抓取
- 🎯 智能分片文件处理
- 📊 NFO 文件生成
- 🖼️ 图片下载和处理
- 🌐 代理支持

## 📄 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## ⚠️ 免责声明

本工具仅用于合法的个人媒体库管理目的。请遵守当地法律法规，尊重版权和隐私。开发者不对使用本工具产生的任何法律问题承担责任。

## 📞 支持与反馈

- 🐛 问题反馈: [GitHub Issues](https://github.com/Feng4/movie_data_capture_go/issues)

## 🙏 致谢

[sqzw-x/mdcx](https://github.com/sqzw-x/mdcx)<br/>
[mvdctop/Movie_Data_Capture](https://github.com/mvdctop/Movie_Data_Capture)


---

**⭐ 如果这个项目对你有帮助，请给它一个 Star！**
