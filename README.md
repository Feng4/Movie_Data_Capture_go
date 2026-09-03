# Movie Data Capture Go

[![Go Version](https://img.shields.io/badge/Go-1.22+-blue.svg)](https://golang.org/)
[![License](https://img.shields.io/badge/License-MIT-green.svg)](LICENSE)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-lightgrey.svg)](#安装)

Movie Data Capture Go（简称 MDC-Go）是一个用 Go 语言编写的电影元数据自动刮削和整理工具。它会从文件名中识别番号，到多个网站抓取影片信息，生成 Kodi / Jellyfin / Emby 兼容的 NFO 元数据文件，下载并处理封面海报，最后按自定义规则把文件整理成规范的媒体库目录结构。

> 本项目是 [mvdctop/Movie_Data_Capture](https://github.com/mvdctop/Movie_Data_Capture)（Python 版）的 Go 重写版，单一可执行文件、无运行时依赖、跨平台。

## 目录

- [功能特性](#功能特性)
- [安装](#安装)
- [快速开始](#快速开始)
- [运行模式](#运行模式)
- [文件处理方式](#文件处理方式)
- [命令行参数](#命令行参数)
- [常用命令示例](#常用命令示例)
- [配置文件详解](#配置文件详解)
- [支持的数据源](#支持的数据源)
- [命名规则与变量](#命名规则与变量)
- [输出结构](#输出结构)
- [分片文件处理](#分片文件处理)
- [常见使用场景](#常见使用场景)
- [故障排除](#故障排除)
- [开发指南](#开发指南)
- [更新日志](#更新日志)

## 功能特性

### 核心功能

- **多站点数据刮削**: 支持 JavBus、FANZA、JavDB、X-City、麻豆区等 24 个数据源（含国产传媒）
- **智能番号解析**: 自动从文件名提取番号，支持自定义正则
- **分片文件处理**: 自动识别 CD1/CD2、Part1/Part2 等多分片文件，分组刮削、一起归档
- **NFO 文件生成**: 生成符合 Kodi/Jellyfin 标准的元数据文件
- **图片下载与处理**: 自动下载封面、海报、剧照，支持人脸识别裁剪
- **文件整理**: 按「演员/番号/标题」等自定义规则整理目录结构和文件名
- **三种处理方式**: 移动、软链接、硬链接，满足不同存储管理需求

### 高级功能

- **水印处理**: 根据影片属性（中字、流出、破解、4K、无码等）自动为海报打标
- **翻译支持**: 将标题、简介翻译为简体中文
- **代理支持**: HTTP / SOCKS5 代理，内置重试与超时控制
- **多线程处理**: 可配置并发数，大批量文件处理更高效
- **失败管理**: 刮削失败的文件自动移入失败目录，支持失败列表跳过
- **映射表缓存**: 缓存番号到元数据的映射，重复运行不重复请求

## 安装

### 方式 1: 下载预编译版本（推荐）

从 [Releases](https://github.com/Feng4/movie_data_capture_go/releases) 页面下载对应系统的压缩包：

| 系统 | 文件 |
|------|------|
| Windows | `mdc-windows-amd64.zip` |
| Linux | `mdc-linux-amd64.tar.gz` |
| macOS (Intel) | `mdc-darwin-amd64.tar.gz` |
| macOS (Apple Silicon) | `mdc-darwin-arm64.tar.gz` |

解压后赋予运行权限（Linux/macOS）:

```bash
chmod +x mdc
```

> Windows 下程序需要 `Img/` 目录中的水印素材，请保持压缩包内目录结构完整。

### 方式 2: 从源码编译

需要 Go 1.22 或更高版本：

```bash
git clone https://github.com/Feng4/movie_data_capture_go.git
cd movie_data_capture_go

# 下载依赖
go mod download

# 编译当前平台
go build -o mdc main.go

# 跨平台编译示例
GOOS=linux   GOARCH=amd64 go build -o mdc-linux   main.go
GOOS=windows GOARCH=amd64 go build -o mdc.exe     main.go
GOOS=darwin  GOARCH=arm64 go build -o mdc-mac-arm main.go
```

### 方式 3: Docker

仓库提供了 `Dockerfile` 和 `docker-compose.yml`：

```bash
# 使用 docker compose（会挂载 ./movies、./JAV_output、./config.yaml 等目录）
docker compose up movie-data-capture
```

`docker-compose.yml` 默认执行 `-path /app/movies`，把电影放入 `./movies` 目录即可。如需代理，取消 `network_mode: "host"` 注释并在 `config.yaml` 中填写宿主机代理地址。

## 快速开始

五步上手：

```bash
# 1. 查看版本，确认程序可运行
./mdc -version

# 2. 首次运行，自动在当前目录生成默认 config.yaml
./mdc

# 3. 编辑 config.yaml，至少确认以下两项
#    common.source_folder  -> 你的电影所在目录
#    proxy.proxy           -> 你的代理地址（可选但强烈推荐）

# 4. 先用单个文件测试
./mdc -file "SSIS-001.mp4"

# 5. 测试无误后，批量处理整个目录
./mdc -path "/path/to/movies"
```

处理成功的文件会出现在 `JAV_output/` 下，并带有 NFO、海报、封面等元数据文件；失败的文件会移入 `failed/`。

## 运行模式

通过 `common.main_mode`（或命令行 `-mode`）选择：

| 模式 | 名称 | 说明 |
|------|------|------|
| 1 | 刮削模式（默认） | 完整流程：抓取数据、下载图片、生成 NFO、移动/链接文件到输出目录 |
| 2 | 整理模式 | 不访问网络，仅按已有数据整理文件结构（适合网络不佳时快速归类） |
| 3 | 分析模式 | 原地刮削：在文件所在目录生成 NFO 和图片，**不移动文件** |

典型用法：

```bash
# 刮削模式批量处理
./mdc -mode 1 -path "/movies"

# 分析模式：给已有媒体库补 NFO，不动文件位置
./mdc -mode 3 -path "/media/library"
```

分析模式还有两个配套项：`common.nfo_skip_days`（NFO 在 N 天内更新过的文件直接跳过）和 `common.ignore_failed_list`（是否忽略失败列表）。

## 文件处理方式

通过 `common.link_mode` 选择刮削成功后文件的去向：

| 值 | 方式 | 说明 |
|----|------|------|
| 0 | 移动（默认） | 把视频文件移动到输出目录 |
| 1 | 软链接 | 在输出目录创建软链接，原文件保持不变 |
| 2 | 硬链接 | 创建硬链接（同盘分区），失败时回退软链接 |

软/硬链接模式适合「保持原始下载目录不动、另建一套整理好的媒体库」的场景。

> Windows 下创建软链接需要管理员权限，或开启系统的开发者模式。

## 命令行参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-config` | 指定配置文件路径 | `-config my.yaml` |
| `-file` | 处理单个文件 | `-file "SSIS-001.mp4"` |
| `-path` | 批量处理指定目录 | `-path "/movies"` |
| `-mode` | 覆盖配置中的运行模式 (1/2/3) | `-mode 3` |
| `-number` | 手动指定番号（覆盖自动识别） | `-number "SSIS-001"` |
| `-search` | 只搜索番号信息，不处理文件 | `-search "SSIS-001"` |
| `-source` | 指定单一数据源 | `-source javbus` |
| `-url` | 指定详情页 URL 直接刮削 | `-url "https://..."` |
| `-debug` | 启用调试模式（详细日志） | `-debug` |
| `-logdir` | 日志输出目录（写入文件） | `-logdir "./logs"` |
| `-version` | 显示版本信息 | `-version` |

## 常用命令示例

```bash
# 处理单个文件，番号自动从文件名识别
./mdc -file "SSIS-001.mp4"

# 文件名识别不出番号时，手动指定
./mdc -file "mystery_video.mp4" -number "SSIS-001"

# 批量处理目录
./mdc -path "/downloads/movies"

# 使用独立配置文件批量处理
./mdc -config jellyfin.yaml -path "/media/nas/share"

# 只搜索看看数据源能查到什么，不落盘处理
./mdc -search "MDX-0212" -source madouqu

# 指定 URL 直接刮削（已知详情页时）
./mdc -file "movie.mp4" -url "https://www.javbus.com/SSIS-001"

# 调试模式排查问题，日志写入文件
./mdc -debug -path "/movies" -logdir "./logs"

# 分析模式批量为现有库补元数据
./mdc -mode 3 -path "/media/library"
```

## 配置文件详解

程序启动时按以下顺序查找配置：`-config` 指定路径 → `./config.yaml` → `./config.yml` → `~/mdc.yaml` 等。找不到时会自动生成默认 `config.yaml`。

完整带注释模板见 [config_template.yaml](config_template.yaml)。各节说明如下。

### common — 通用配置

```yaml
common:
  main_mode: 1                  # 运行模式: 1=刮削, 2=整理, 3=分析
  source_folder: "./"           # 源文件夹（-path 未指定时使用）
  success_output_folder: "JAV_output"  # 成功输出目录
  failed_output_folder: "failed"       # 失败输出目录
  link_mode: 0                  # 0=移动, 1=软链接, 2=硬链接
  failed_move: true             # 失败文件移入失败目录
  del_empty_folder: true        # 处理后删除源目录中的空文件夹
  multi_threading: 0            # 并发数，0=顺序处理（建议 2~4）
  sleep: 3                      # 每个文件处理间隔秒数（防封）
  stop_counter: 0              # 处理 N 个后停止，0=不限
  nfo_skip_days: 30            # 模式3：NFO 更新于 N 天内的跳过
  ignore_failed_list: false     # 忽略失败列表（默认跳过曾失败的文件）
  download_only_missing_images: true  # 只下载缺失图片，避免重复下载
  mapping_table_validity: 7     # 映射表缓存有效期（天）
  actor_gender: "female"        # 演员性别过滤: female, male, both, all
  actor_only_tag: false         # 标签只保留演员名
  jellyfin: 0                   # Jellyfin 兼容模式（0=通用）
  anonymous_fill: 0             # 未知演员填充方式
  translate_to_sc: true         # 繁转简
```

### proxy — 网络代理

```yaml
proxy:
  switch: true                  # 启用代理（强烈推荐）
  proxy: "127.0.0.1:10808"      # 代理地址
  type: "socks5"                # http / socks5 / socks5h
  timeout: 30                   # 超时（秒）
  retry: 5                      # 重试次数
  cacert_file: ""               # 自定义 CA 证书路径（可选）
```

### name_rule — 命名规则

```yaml
name_rule:
  location_rule: "actor + '/' + number"   # 目录规则
  naming_rule: "number + '-' + title"     # 文件名规则
  max_title_len: 50                       # 标题最大长度
  image_naming_with_number: false         # 图片文件名带番号前缀
  number_uppercase: false                 # 番号转大写
  number_regexs: ""                       # 自定义番号正则
```

可用变量见 [命名规则与变量](#命名规则与变量)。

### priority — 数据源优先级

```yaml
priority:
  website: "javbus,fanza,fc2,fc2club,javdb,xcity,mgstage,jav321,madouqu"
```

按顺序依次尝试，直到某个源成功返回数据。完整数据源列表见[下一节](#支持的数据源)。`madouqu` 为国产源，放在末位不影响日系番号的检索速度。

### escape — 转义配置

```yaml
escape:
  literals: "\\()/ "          # 路径中需替换的字符
  folders: "failed, JAV_output"  # 扫描时跳过的目录
```

### media — 媒体类型

```yaml
media:
  media_type: ".mp4,.avi,.rmvb,.wmv,.mov,.mkv,.flv,.ts,.webm,.iso"
  sub_type: ".smi,.srt,.idx,.sub,.sup,.psb,.ssa,.ass,.usf,.xss,.ssf,.rt,.lrc,.sbv,.vtt,.ttml"
```

小于 120MB 的视频文件默认视为广告跳过（调试模式下会处理）。

### 其余功能开关

```yaml
watermark:                     # 水印（中字/流出/破解/4K/无码等角标）
  switch: true
  water: 2                     # 位置: 1=左上, 2=右上, 3=左下, 4=右下

extrafanart:                   # 额外剧照
  switch: true
  extrafanart_folder: "extrafanart"
  parallel_download: 1

trailer:                       # 预告片下载
  switch: false

face:                          # 人脸识别裁剪海报
  locations_model: "hog"       # hog / cnn
  uncensored_only: true        # 仅无码影片执行
  always_imagecut: false       # 总是裁剪
  aspect_ratio: 2.12

translate:                     # 翻译
  switch: false
  engine: "google-free"
  target_language: "zh_cn"
  values: "title,outline"
  delay: 1

storyline:                     # 剧情简介来源
  switch: true
  site: "1:avno1"
  censored_site: "5:xcity,6:amazon"
  uncensored_site: "3:58avgo"

uncensored:
  uncensored_prefix: "S2M,BT,LAF,SMD"  # 无码番号前缀

javdb:
  sites: "38,39"               # JavDB 站点后缀

jellyfin:
  multi_part_fanart: false     # 多分片封面合并

actor_photo:
  download_for_kodi: false     # 为 Kodi 下载演员头像

debug_mode:
  switch: false                # 调试模式（DEBUG 级日志）
```

## 支持的数据源

共 24 个。`默认` 列标注是否在 `priority.website` 中默认启用。

| 配置名 | 网站 | 类型 | 默认 |
|--------|------|------|------|
| `javbus` | JavBus | 综合 | ✅ |
| `fanza` | FANZA | 官方 | ✅ |
| `fc2` / `fc2club` | FC2 / FC2Club | 素人 | ✅ |
| `javdb` | JavDB | 综合 | ✅ |
| `xcity` | X-City | 无码 | ✅ |
| `mgstage` | MGStage | 素人企划 | ✅ |
| `jav321` | JAV321 | 综合 | ✅ |
| `madouqu` / `mdq` | 麻豆区 | 国产传媒 | ✅ |
| `dmm` | DMM | 官方 | |
| `javlibrary` | JavLibrary | 综合 | |
| `javmenu` | JavMenu | 综合 | |
| `freejavbt` | FreeJavBT | 综合 | |
| `cableav` | CableAV | 综合 | |
| `carib` / `caribbeancom` | Caribbeancom | 无码 | |
| `caribpr` / `caribbeancompr` | CaribbeancomPR | 无码 | |
| `dahlia` | Dahlia | 厂牌 | |
| `faleno` | Faleno | 厂牌 | |
| `fantastica` | Fantastica | 厂牌 | |
| `dlsite` | DLsite | 同人 | |
| `gcolle` | GColle | 同人 | |
| `getchu` | Getchu | 动漫 | |
| `cnmdb` | CNMDB | 国产 | |
| `javday` | JavDay | 国产 | |
| `madou` / `md` | 麻豆 | 国产 | |

启用非默认源：把配置名追加进 `priority.website` 即可，例如：

```yaml
priority:
  website: "javbus,javdb,fanza,carib,getchu"
```

临时使用单一数据源（不改配置）：

```bash
./mdc -source madouqu -number "MDX-0212"
```

## 命名规则与变量

`name_rule.location_rule` 控制目录层级，`naming_rule` 控制文件名。可用变量：

| 变量 | 含义 |
|------|------|
| `actor` | 演员名 |
| `number` | 番号 |
| `title` | 标题 |
| `year` | 发行年份 |
| `studio` | 制作商 |
| `label` | 系列 |

示例：

```yaml
# 目录: 演员名/番号，文件名: 番号-标题
location_rule: "actor + '/' + number"
naming_rule: "number + '-' + title"
# => JAV_output/葵つかさ/SSIS-001/SSIS-001-美しい人妻の秘密.mp4

# 按制作商归档
location_rule: "studio + '/' + actor + '/' + number"
naming_rule: "actor + '_' + number + '_' + title"
# => JAV_output/S1_NO.1_STYLE/葵つかさ/SSIS-001/葵つかさ_SSIS-001_美しい人妻の秘密.mp4
```

文件名中的 `\/:*?"<>|` 等非法字符会自动替换，超长标题按 `max_title_len` 截断。

## 输出结构

刮削模式处理完成后，输出目录结构如下：

```
JAV_output/
├── 演员名/
│   ├── SSIS-001/                     # location_rule 生成
│   │   ├── SSIS-001.mp4              # 视频文件
│   │   ├── SSIS-001.nfo              # 元数据（Kodi/Jellyfin/Emby）
│   │   ├── fanart.jpg                # 背景图
│   │   ├── poster.jpg                # 海报（含水印角标）
│   │   ├── thumb.jpg                 # 缩略图
│   │   └── extrafanart/              # 额外剧照
│   │       ├── extrafanart-1.jpg
│   │       └── extrafanart-2.jpg
failed/
└── 无法识别或刮削失败的文件
```

影片属性会以角标水印体现在海报上（`Img/` 目录提供素材）：

| 文件名标志 | 含义 |
|-----------|------|
| `-C` | 内嵌中文字幕 |
| `-leak` | 流出版本 |
| `-hack` | 破解版 |
| `4K` | 4K 分辨率 |
| 无码前缀 | 无码影片 |

## 分片文件处理

同一影片分成多个文件时，程序会自动分组，只刮削一次、归档到同一目录：

| 格式 | 示例 |
|------|------|
| CD | `SSIS-001-cd1.mp4`, `SSIS-001-cd2.mp4` |
| Part | `SSIS-001_part_1.mp4`, `SSIS-001_part_2.mp4` |
| 数字 | `SSIS-001_1.mp4`, `SSIS-001_2.mp4` |
| 字母 | `SSIS-001-A.mp4`, `SSIS-001-B.mp4` |
| Disc | `SSIS-001-disc1.mkv`, `SSIS-001-disc2.mkv` |

归档后统一命名为 `番号-1`、`番号-2`…（带属性后缀），NFO 中记录总分片数与各分片信息。

## 常见使用场景

### 场景 1: 初次整理下载目录

```yaml
common:
  main_mode: 1
  link_mode: 0          # 移动文件
  del_empty_folder: true
```

### 场景 2: 保留原始文件，另建媒体库视图

```yaml
common:
  main_mode: 1
  link_mode: 1          # 软链接
```

### 场景 3: 给现有媒体库补元数据（不动文件）

```yaml
common:
  main_mode: 3          # 分析模式，原地生成 NFO
  nfo_skip_days: 30     # 跳过近期已处理的
```

### 场景 4: 弱网环境仅归类

```yaml
common:
  main_mode: 2          # 整理模式，不联网
```

### 场景 5: 大批量处理

```yaml
common:
  multi_threading: 4    # 并发
  sleep: 1              # 降低间隔
  stop_counter: 200     # 分批控制
```

## 故障排除

| 现象 | 处理方法 |
|------|---------|
| 连接超时 / 抓取失败 | 检查代理配置（`proxy.switch`、地址、类型）；增大 `timeout` 与 `retry` |
| 番号识别失败 | 文件名尽量规范；用 `-number` 手动指定；配置 `name_rule.number_regexs` 自定义正则 |
| NFO 在媒体中心不显示 | 确认 NFO 与视频同名同目录；检查媒体中心刮削器设置 |
| 分片未正确分组 | 按[支持格式](#分片文件处理)重命名；用 `-debug` 查看分组日志 |
| Windows 软链接失败 | 以管理员运行，或改用 `link_mode: 2`（硬链接） |
| 小体积正片被跳过 | 小于 120MB 的文件默认视为广告，可加 `-debug` 强制处理 |
| 图片下载不完整 | 开启 `download_only_missing_images: true` 后重跑即可补齐 |

更多内容见 [Wiki 文档](docs/wiki/Home.md)、[USER_MANUAL.md](USER_MANUAL.md) 与 [QUICK_START.md](QUICK_START.md)。

## 开发指南

```bash
# 运行全部测试
go test ./...

# 查看覆盖率
go test ./... -cover

# 单独跑某个数据源的测试
go test ./internal/scraper/ -run MadouQu -v

# 静态检查
go vet ./...
```

> Windows 上并行跑 `go test ./...` 偶发因资源竞争超时，可加 `-p 1` 串行执行。

已具备单元测试的包：`pkg/retry`（91%）、`pkg/imageprocessor`（75%）、`pkg/parser`（71%）、`pkg/fragment`（69%）、`pkg/performance`（57%）、`internal/config`（33%）、`pkg/recovery`（33%）、`internal/scraper`（madouqu 数据源解析）。

### 项目结构

```
movie-data-capture/
├── main.go               # 程序入口
├── config.yaml           # 配置文件
├── config_template.yaml  # 配置模板（含完整注释说明）
├── internal/             # 内部包
│   ├── config/           # 配置管理
│   ├── core/             # 核心处理逻辑
│   └── scraper/          # 数据抓取器（24 个数据源）
├── pkg/                  # 公共包
│   ├── downloader/       # 下载器
│   ├── facedetection/    # 人脸检测
│   ├── fragment/         # 分片文件处理
│   ├── httpclient/       # HTTP 客户端
│   ├── imageprocessor/   # 图片处理（裁剪、增强）
│   ├── logger/           # 日志系统
│   ├── nfo/              # NFO 生成器
│   ├── parser/           # 番号解析器
│   ├── performance/      # 性能监控与并发工具
│   ├── recovery/         # 错误恢复与状态持久化
│   ├── retry/            # 智能重试机制
│   ├── storage/          # 存储管理
│   ├── utils/            # 工具函数
│   └── watermark/        # 水印处理
├── Img/                  # 水印素材
└── MappingTable/         # 番号映射数据
```

### 贡献

1. **报告问题**: [Issues](https://github.com/Feng4/movie_data_capture_go/issues)
2. **功能建议 / 代码贡献**: 提 Pull Request
3. **文档改进**: 完善 README 与示例

## 更新日志

### v1.1.1 (2026-09-03)

- 🗑️ 移除 STRM 文件生成功能（含 `pkg/strm` 包、`strm` 配置节及相关文档）
- 📝 重写 README，补充完整的安装、模式、参数、配置与故障排除说明
- 📚 新增 Wiki 文档（`docs/wiki/`，共 10 页：安装、快速开始、CLI、配置、模式、数据源、命名、分片、故障排除）
- ⚙️ 发布流程改进：CI 先跑测试再构建、产物自校验版本号、Release 说明自动提取自 README 更新日志，发布包附带 `docs/` 与 `config_template.yaml`
- 🐛 CI 修复：交叉编译产物跳过本机自检（Exec format error）；macOS runner 因 dyld 兼容问题跳过测试步骤（测试由 Linux/Windows 覆盖），工具链升级至 Go 1.25

### v1.1.0 (2026-08-01)

- ✨ 新增 **麻豆区（madouqu）** 国产传媒数据源，已加入默认源列表
- 🧹 清理仓库：移除误提交的编译产物与根目录临时调试脚本，新增 `.gitignore`
- 🐛 修复 `pkg/performance` 工作池 `Stop()` 死锁（未取消 context 时会永久阻塞）
- 🐛 修复 `pkg/recovery` 状态保存自死锁与零值配置陷阱（`RecoveryTimeout=0` 导致上下文立即超时）
- 🐛 修复 `GetStats()`/`GetMetrics()` 拷贝含锁结构体的并发缺陷（`go vet` copylocks）
- ✅ `go build ./...`、`go vet ./...` 与全部测试通过

### v1.0.0 (2025-08-30)

- ✨ 首个正式版本发布
- 🚀 支持多站点数据抓取
- 🎯 智能分片文件处理
- 📊 NFO 文件生成
- 🖼️ 图片下载和处理
- 🌐 代理支持

## 许可证

本项目采用 MIT 许可证 - 查看 [LICENSE](LICENSE) 文件了解详情。

## 免责声明

本工具仅用于合法的个人媒体库管理目的。请遵守当地法律法规，尊重版权和隐私。开发者不对使用本工具产生的任何法律问题承担责任。

## 致谢

[sqzw-x/mdcx](https://github.com/sqzw-x/mdcx)
[mvdctop/Movie_Data_Capture](https://github.com/mvdctop/Movie_Data_Capture)

---

**⭐ 如果这个项目对你有帮助，请给它一个 Star！**
