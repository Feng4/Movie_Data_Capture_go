# Movie Data Capture - Go Implementation

这是原Python版本Movie Data Capture工具的Go语言完整重写版本，保持了原有功能的完整性，同时利用Go语言的特性提供了更好的性能和并发处理能力。

## 🚀 项目改进进度

### 从Python版本迁移的主要改进点

#### 1. 架构重构
- ✅ **模块化设计**: 采用清晰的包结构，分离关注点
- ✅ **依赖注入**: 使用接口和依赖注入提高可测试性
- ✅ **配置管理**: 从INI格式迁移到YAML，支持更复杂的配置结构
- ✅ **错误处理**: 统一的错误处理机制，支持错误包装和上下文
- ✅ **测试覆盖**: 核心工具包具备单元测试（retry 91%、imageprocessor 75%、parser 71%、fragment 69%）

#### 2. 性能优化
- ✅ **并发模型**: 使用goroutines替代线程池，支持更高的并发度
- ✅ **内存管理**: 优化内存使用，减少GC压力，使用对象池和内存池
- ✅ **网络优化**: HTTP连接池复用，减少连接开销，支持重试机制
- ✅ **I/O优化**: 异步文件操作，提高磁盘I/O效率
- ✅ **错误恢复**: 智能重试和失败处理机制

#### 3. 功能增强
- ✅ **日志系统**: 结构化日志，支持颜色输出和级别控制
- ✅ **图片处理**: 完整的图片裁剪和水印功能，支持多种格式
- ✅ **番号识别**: 自定义正则表达式支持，兼容更多番号格式
- ✅ **错误恢复**: 更好的错误恢复和重试机制
- ✅ **多数据源**: 支持 24 个数据源，涵盖主流、小众与国产站点

### 当前已实现的核心功能模块

| 模块 | 状态 | 功能描述 | 兼容性 |
|------|------|----------|--------|
| 🔍 **数据爬取** | ✅ 完成 | 支持 24 个数据源，包括JavDB、JavBus、Fanza、麻豆区等 | 100% |
| 📁 **文件处理** | ✅ 完成 | 文件扫描、移动、重命名、目录创建 | 100% |
| 🖼️ **图片处理** | ✅ 完成 | 封面下载、裁剪、水印添加、剧照处理 | 100% |
| 📄 **NFO生成** | ✅ 完成 | Kodi/Jellyfin兼容的元数据文件生成 | 100% |
| 🔧 **配置管理** | ✅ 完成 | YAML配置文件，支持热重载 | 100% |
| 📊 **日志系统** | ✅ 完成 | 结构化日志，多级别输出，颜色支持 | 100% |
| 🌐 **网络处理** | ✅ 完成 | 代理支持、重试机制、超时控制 | 100% |
| 🔢 **番号识别** | ✅ 完成 | 内置模式+自定义正则表达式 | 100% |
| 🔄 **错误处理** | ✅ 完成 | 失败文件管理、重试机制、错误恢复 | 100% |

### 性能优化和架构调整情况

#### 并发处理优化
```go
// 示例：并发文件处理
func (p *Processor) ProcessFiles(files []string) error {
    semaphore := make(chan struct{}, p.config.MaxConcurrency)
    var wg sync.WaitGroup
    
    for _, file := range files {
        wg.Add(1)
        go func(f string) {
            defer wg.Done()
            semaphore <- struct{}{} // 获取信号量
            defer func() { <-semaphore }() // 释放信号量
            
            p.processFile(f)
        }(file)
    }
    
    wg.Wait()
    return nil
}
```

#### 内存优化
- **流式处理**: 大文件下载使用流式处理，避免内存溢出
- **对象池**: 复用HTTP客户端和缓冲区
- **及时释放**: 主动释放不再使用的资源

#### 网络优化
```go
// HTTP客户端配置
var httpClient = &http.Client{
    Timeout: 30 * time.Second,
    Transport: &http.Transport{
        MaxIdleConns:        100,
        MaxIdleConnsPerHost: 10,
        IdleConnTimeout:     90 * time.Second,
    },
}
```

## 主要功能

- **影视信息爬取**: 从多个网站爬取电影元数据信息
- **封面和剧照下载**: 并行下载封面图片和剧照
- **NFO文件生成**: 生成Kodi/Jellyfin兼容的NFO元数据文件
- **文件组织**: 根据规则自动组织文件和创建目录结构
- **多站点支持**: 支持 JavDB、JavBus、Fanza、XCity、麻豆区等 24 个数据源
- **并发处理**: 利用Go协程实现高效的并发处理
- **错误处理**: 完善的错误处理和重试机制

## 项目架构

```
movie-data-capture/
├── main.go                    # 主程序入口
├── go.mod                     # Go模块定义
├── go.sum                     # Go依赖校验
├── config.yaml               # 配置文件
├── config_template.yaml      # 配置模板（含完整注释说明）
├── internal/                 # 内部包
│   ├── config/               # 配置管理
│   │   └── config.go
│   ├── core/                 # 核心处理逻辑
│   │   └── processor.go
│   └── scraper/              # 数据爬取模块
│       ├── scraper.go        # 爬虫核心接口与分派
│       ├── javbus.go         # JavBus数据源
│       ├── javdb.go          # JavDB数据源
│       ├── improved_javdb.go # JavDB增强版数据源
│       ├── fanza.go          # Fanza数据源
│       ├── dmm.go            # DMM数据源
│       ├── xcity.go          # XCity数据源
│       ├── mgstage.go        # MGStage数据源
│       ├── fc2.go            # FC2数据源
│       ├── fc2club.go        # FC2Club数据源
│       ├── jav321.go         # JAV321数据源
│       ├── javlibrary.go     # JavLibrary数据源
│       ├── cableav.go        # CableAV数据源
│       ├── cnmdb.go          # CNMDB数据源
│       ├── dahlia.go         # Dahlia数据源
│       ├── faleno.go         # Faleno数据源
│       ├── fantastica.go     # Fantastica数据源
│       ├── carib.go          # Caribbeancom数据源
│       ├── caribpr.go        # CaribbeancomPR数据源
│       ├── dlsite.go         # DLsite数据源
│       ├── gcolle.go         # GColle数据源
│       ├── getchu.go         # Getchu数据源
│       ├── javmenu.go        # JavMenu数据源
│       ├── javday.go         # JavDay数据源
│       ├── freejavbt.go      # FreeJavBT数据源
│       ├── madou.go          # 麻豆数据源
│       ├── madouqu.go        # 麻豆区数据源（国产传媒）
│       └── utils.go          # 爬虫工具函数
└── pkg/                      # 公共包
    ├── downloader/           # 文件下载
    │   └── downloader.go
    ├── facedetection/        # 人脸检测
    │   └── facedetection.go
    ├── fragment/             # 分片文件处理
    │   └── fragment.go
    ├── httpclient/           # HTTP客户端
    │   └── client.go
    ├── imageprocessor/       # 图片处理（裁剪、增强、水印）
    │   ├── imageprocessor.go
    │   └── enhancement.go
    ├── logger/               # 日志系统
    │   └── logger.go
    ├── nfo/                  # NFO文件生成
    │   └── generator.go
    ├── parser/               # 番号解析
    │   └── number_parser.go
    ├── performance/          # 性能监控与并发工具
    │   ├── monitor.go
    │   ├── concurrency.go
    │   ├── memory.go
    │   └── network.go
    ├── recovery/             # 错误恢复与状态持久化
    │   ├── recovery.go
    │   └── strategies.go
    ├── retry/                # 智能重试机制
    │   └── retry.go
    ├── storage/              # 存储管理
    │   └── storage.go
    ├── utils/                # 工具函数
    │   └── utils.go
    └── watermark/            # 水印处理
        ├── watermark.go
        └── advanced_watermark.go
```

### 架构说明

#### 核心模块
- **main.go**: 程序入口，处理命令行参数和程序初始化
- **internal/config**: 配置文件解析和管理，支持YAML格式
- **internal/core**: 核心业务逻辑，文件处理流程控制
- **internal/scraper**: 多数据源爬虫实现，支持 24 个主要站点

#### 公共包
- **pkg/httpclient**: HTTP客户端封装，支持代理和重试
- **pkg/logger**: 结构化日志系统，支持颜色输出
- **pkg/nfo**: NFO元数据文件生成
- **pkg/storage**: 文件存储和目录管理
- **pkg/utils**: 通用工具函数，包括番号识别
- **pkg/watermark**: 图片水印处理功能

#### 数据源支持
| 数据源 | 配置名 | 文件 | 说明 |
|--------|--------|------|------|
| JavDB | `javdb` | improved_javdb.go | 综合，主力源 |
| JavBus | `javbus` | javbus.go | 综合，覆盖面广 |
| Fanza | `fanza` | fanza.go | 官方数据，质量高 |
| DMM | `dmm` | dmm.go | 官方数据 |
| XCity | `xcity` | xcity.go | 无码作品 |
| MGStage | `mgstage` | mgstage.go | 素人企划 |
| FC2 / FC2Club | `fc2` / `fc2club` | fc2club.go | 素人作品 |
| JAV321 | `jav321` | jav321.go | 综合 |
| JavLibrary | `javlibrary` | javlibrary.go | 综合 |
| CableAV | `cableav` | cableav.go | 综合 |
| CNMDB | `cnmdb` | cnmdb.go | 国产 |
| Dahlia | `dahlia` | dahlia.go | 厂牌专用 |
| Faleno | `faleno` | faleno.go | 厂牌专用 |
| Fantastica | `fantastica` | fantastica.go | 厂牌专用 |
| Caribbeancom | `carib` / `caribbeancom` | carib.go | 无码 |
| CaribbeancomPR | `caribpr` / `caribbeancompr` | caribpr.go | 无码 |
| DLsite | `dlsite` | dlsite.go | 同人作品 |
| GColle | `gcolle` | gcolle.go | 同人作品 |
| Getchu | `getchu` | getchu.go | 动漫作品 |
| JavMenu | `javmenu` | javmenu.go | 综合 |
| JavDay | `javday` | javday.go | 国产 |
| FreeJavBT | `freejavbt` | freejavbt.go | 综合 |
| 麻豆 | `madou` / `md` | madou.go | 国产 |
| 麻豆区 | `madouqu` / `mdq` | madouqu.go | 国产传媒（麻豆/天美/蜜桃等厂牌） |


## 🔄 与Python版本对比分析

### 开发效率对比

| 方面 | Python版本 | Go版本 | 说明 |
|------|------------|--------|----- |
| **开发速度** | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | Python语法更简洁 |
| **调试难度** | ⭐⭐⭐ | ⭐⭐⭐⭐ | Go编译时错误检查 |
| **重构安全性** | ⭐⭐ | ⭐⭐⭐⭐⭐ | 静态类型系统优势 |
| **测试覆盖** | ⭐⭐⭐ | ⭐⭐⭐⭐ | Go内置测试框架 |
| **文档生成** | ⭐⭐⭐ | ⭐⭐⭐⭐ | godoc自动生成 |



### 部署和维护便利性

#### 部署对比
| 特性 | Python版本 | Go版本 |
|------|------------|--------|
| **部署文件** | 源码+依赖 | 单一可执行文件 |
| **环境要求** | Python 3.8+ | 无 |
| **依赖安装** | pip install | 无需安装 |
| **跨平台** | 需要对应平台Python | 编译时指定目标平台 |
| **容器大小** | ~500MB | ~20MB |

#### 维护成本
```bash
# Python版本维护任务
- 定期更新Python版本
- 管理虚拟环境
- 解决依赖冲突
- 处理平台兼容性问题

# Go版本维护任务
- 定期更新Go版本（向后兼容性好）
- 更新依赖（go mod tidy）
- 重新编译发布
```

## 安装和使用

### 前置要求
- Go 1.22 或更高版本

### 安装依赖
```bash
go mod tidy
```

### 运行测试
```bash
# 运行全部测试
go test ./...

# 查看各包覆盖率
go test ./... -cover

# 运行单个包的测试（例如新增的 madouqu 数据源）
go test ./internal/scraper/ -run MadouQu -v
```

> Windows 上并行跑 `go test ./...` 偶发因资源竞争超时，可加 `-p 1` 串行执行。

### 基本使用
```bash
# 显示帮助
go run main.go --help

# 处理单个文件
go run main.go --file "/path/to/movie.mp4"

# 处理文件夹
go run main.go --path "/path/to/movies" --mode 1

# 搜索模式
go run main.go --search "STAR-123"

# 调试模式
go run main.go --debug --path "/path/to/movies"
```

### 编译可执行文件
```bash
# 编译当前平台
go build -o movie-data-capture main.go

# 跨平台编译
GOOS=linux GOARCH=amd64 go build -o movie-data-capture-linux main.go
GOOS=windows GOARCH=amd64 go build -o movie-data-capture.exe main.go
GOOS=darwin GOARCH=amd64 go build -o movie-data-capture-mac main.go
```

## 配置说明

配置文件使用YAML格式，主要配置项：

```yaml
common:
  main_mode: 1                    # 1=刮削模式, 2=整理模式, 3=分析模式
  source_folder: "./"             # 源文件夹
  success_output_folder: "JAV_output"  # 成功输出文件夹
  multi_threading: 5              # 并发线程数
  
proxy:
  switch: false                   # 是否启用代理
  proxy: "127.0.0.1:1080"        # 代理地址
  type: "socks5"                 # 代理类型
  
priority:
  # 按顺序尝试，madouqu 为国产源，置于末位不影响日系番号的检索速度
  website: "javbus,fanza,fc2,fc2club,javdb,xcity,mgstage,jav321,madouqu"
```

也可在运行时用 `-source` 指定单一数据源：

```bash
# 只用麻豆区抓取国产番号
./mdc -source madouqu -number "MDX-0212"
```
