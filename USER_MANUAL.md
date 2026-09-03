# Movie Data Capture Go 使用手册

## 📖 目录

1. [简介](#简介)
2. [安装指南](#安装指南)
3. [快速开始](#快速开始)
4. [配置详解](#配置详解)
5. [使用模式](#使用模式)
6. [高级功能](#高级功能)
7. [命令行参数](#命令行参数)
8. [故障排除](#故障排除)
9. [最佳实践](#最佳实践)
10. [常见问题](#常见问题)

---

## 简介

Movie Data Capture Go（简称MDC-Go）是一个强大的电影元数据自动抓取和整理工具。它能够：

- 🔍 **智能识别**: 自动从文件名提取番号信息
- 🌐 **多站点抓取**: 从多个数据源获取完整的电影信息
- 📝 **NFO生成**: 创建符合Kodi/Jellyfin标准的元数据文件
- 🖼️ **图片处理**: 下载封面、海报、剧照并自动处理
- 📁 **智能整理**: 按照自定义规则整理文件目录结构
- 🔗 **多种模式**: 支持移动、软链接、硬链接三种文件处理方式
- 🎭 **分片处理**: 智能处理多部分电影文件

---

## 安装指南

### 系统要求

- **操作系统**: Windows 10+、Linux、macOS
- **Go版本**: 1.21 或更高版本（仅源码编译）
- **网络**: 稳定的网络连接（建议配置代理）
- **存储**: 至少1GB可用空间

### 方法1：下载预编译版本

1. 访问 [GitHub Releases](https://github.com/Feng4/movie_data_capture_go/releases)
2. 根据操作系统选择对应版本：
   - Windows: `mdc-windows-amd64.zip`
   - Linux: `mdc-linux-amd64.tar.gz`
   - macOS: `mdc-darwin-amd64.tar.gz` 或 `mdc-darwin-arm64.tar.gz`（M1/M2芯片）

3. 解压到任意目录
4. 确保可执行文件有运行权限（Linux/macOS）：
   ```bash
   chmod +x mdc
   ```

### 方法2：源码编译

```bash
# 克隆仓库
git clone https://github.com/Feng4/movie_data_capture_go.git
cd movie_data_capture_go

# 下载依赖
go mod download

# 编译程序
go build -o mdc main.go

# 运行程序
./mdc --version
```

### 验证安装

```bash
./mdc --version
# 输出: Movie Data Capture Go Version 1.1.0
```

---

## 快速开始

### 第一次运行

1. **初始化配置**：
   ```bash
   ./mdc
   # 程序会自动创建默认的config.yaml配置文件
   ```

2. **编辑配置文件**：
   ```yaml
   # 基本配置示例
   common:
     main_mode: 1                    # 刮削模式
     source_folder: "./movies"       # 源文件夹
     link_mode: 0                    # 移动文件模式
   
   proxy:
     switch: true                    # 启用代理（推荐）
     proxy: "127.0.0.1:10808"       # 代理地址
     type: "socks5"                 # 代理类型
   ```

3. **处理第一个文件**：
   ```bash
   ./mdc -file "SSIS-001.mp4"
   ```

### 基本使用流程

1. **准备电影文件**: 确保文件名包含可识别的番号
2. **配置程序**: 根据需求修改`config.yaml`
3. **运行程序**: 使用命令行参数控制行为
4. **查看结果**: 检查生成的NFO文件和整理后的目录结构

---

## 配置详解

### 配置文件结构

配置文件`config.yaml`包含以下主要部分：

```yaml
# 通用配置
common:
  main_mode: 1                      # 运行模式
  source_folder: "./"               # 源文件夹
  success_output_folder: "JAV_output"  # 成功输出文件夹
  failed_output_folder: "failed"   # 失败输出文件夹
  link_mode: 0                      # 文件处理模式

# 网络代理配置
proxy:
  switch: true                      # 启用代理
  proxy: "127.0.0.1:10808"         # 代理地址
  type: "socks5"                   # 代理类型
  timeout: 30                       # 超时时间
  retry: 5                          # 重试次数

# 命名规则配置
name_rule:
  location_rule: "actor + '/' + number"     # 目录结构规则
  naming_rule: "number + '-' + title"       # 文件命名规则
  max_title_len: 50                         # 最大标题长度

# 网站优先级配置
priority:
  website: "javbus,javdb,fanza,xcity,mgstage"

# 调试模式配置
debug_mode:
  switch: false                     # 启用调试模式

# 其他功能配置...
```

### 核心配置参数详解

#### 1. 运行模式 (main_mode)

```yaml
common:
  main_mode: 1    # 1=刮削, 2=整理, 3=分析
```

- **模式1（刮削）**: 完整的刮削流程，下载图片，生成NFO，整理文件
- **模式2（整理）**: 仅整理文件，不刮削数据
- **模式3（分析）**: 仅生成NFO，不移动文件

#### 2. 文件处理模式 (link_mode)

```yaml
common:
  link_mode: 0    # 0=移动, 1=软链接, 2=硬链接
```

- **0（移动）**: 将文件移动到输出目录
- **1（软链接）**: 创建软链接，原文件保持不变
- **2（硬链接）**: 创建硬链接，失败时回退到软链接

#### 3. 命名规则配置

```yaml
name_rule:
  location_rule: "actor + '/' + number"      # 目录结构
  naming_rule: "number + '-' + title"        # 文件命名
  max_title_len: 50                          # 标题长度限制
  number_uppercase: false                    # 番号大写
```

**可用变量**:
- `actor`: 演员名
- `number`: 番号
- `title`: 标题
- `year`: 年份
- `studio`: 制作商
- `label`: 系列

**示例**:
```yaml
location_rule: "studio + '/' + actor + '/' + number"
# 输出: S1_NO.1_STYLE/葵つかさ/SSIS-001/

naming_rule: "actor + '_' + number + '_' + title"
# 输出: 葵つかさ_SSIS-001_美しい人妻の秘密.mp4
```

#### 4. 网络代理配置

```yaml
proxy:
  switch: true                      # 启用代理
  proxy: "127.0.0.1:10808"         # 代理地址:端口
  type: "socks5"                    # 代理类型
  timeout: 30                       # 超时时间(秒)
  retry: 5                          # 重试次数
```

**支持的代理类型**:
- `http`: HTTP代理
- `socks5`: SOCKS5代理
- `socks5h`: SOCKS5代理(DNS通过代理解析)

#### 5. 数据源优先级

```yaml
priority:
  website: "javbus,javdb,fanza,xcity,mgstage,fc2,fc2club"
```

程序会按照配置的顺序尝试从各个网站获取数据，直到成功。

**支持的数据源**:
- `javbus`: JavBus (主要推荐)
- `javdb`: JavDB
- `fanza`: FANZA (官方数据，质量高)
- `fc2`: FC2 (素人作品)
- `xcity`: X-City (无码作品)
- `mgstage`: MGStage
- 其他...

---

## 使用模式

### 模式1：完整刮削模式

**特点**: 完整的数据抓取、图片下载、NFO生成、文件整理

**配置**:
```yaml
common:
  main_mode: 1
  link_mode: 0                      # 移动文件
```

**使用场景**: 
- 初次整理电影库
- 需要完整元数据的场景
- 希望重新组织文件结构

**示例**:
```bash
# 处理单个文件
./mdc -file "SSIS-001.mp4"

# 批量处理目录
./mdc -path "/path/to/movies"
```

### 模式2：文件整理模式

**特点**: 仅整理文件，不获取网络数据

**配置**:
```yaml
common:
  main_mode: 2
```

**使用场景**:
- 文件已有正确命名，仅需整理
- 网络条件不好的情况
- 快速整理大量文件

### 模式3：分析模式

**特点**: 生成NFO但不移动文件，适合预览效果

**配置**:
```yaml
common:
  main_mode: 3
```

**使用场景**:
- 测试配置效果
- 仅需NFO文件
- 不想改变原始文件位置

### 软链接模式

**特点**: 创建软链接，原文件位置不变

**配置**:
```yaml
common:
  main_mode: 1
  link_mode: 1                      # 软链接
```

**优势**:
- 节省存储空间
- 保持原始文件不变
- 可以创建多个媒体库视图

**示例输出结构**:
```
原始位置:
/movies/raw/SSIS-001.mp4

整理后:
/movies/organized/葵つかさ/SSIS-001-美しい人妻の秘密/
├── SSIS-001-美しい人妻の秘密.mp4 -> /movies/raw/SSIS-001.mp4
├── SSIS-001-美しい人妻の秘密.nfo
├── poster.jpg
└── fanart.jpg
```

---

## 高级功能

### 1. 分片文件处理

**功能**: 自动识别和处理多部分电影文件

**支持的格式**:
- CD格式: `SSIS-001-cd1.mp4`, `SSIS-001-cd2.mp4`
- Part格式: `SSIS-001_part_1.mp4`, `SSIS-001_part_2.mp4`
- 数字格式: `SSIS-001_1.mp4`, `SSIS-001_2.mp4`
- 字母格式: `SSIS-001-A.mp4`, `SSIS-001-B.mp4`
- Disc格式: `SSIS-001-disc1.mkv`, `SSIS-001-disc2.mkv`

**处理效果**: 同一分片组的所有文件会被刮削一次，并移动到同一输出目录，按 `番号-1`、`番号-2` 顺序命名。

### 2. 图像处理功能

#### 人脸识别裁剪
```yaml
face:
  locations_model: "hog"            # 人脸检测模型
  uncensored_only: true             # 仅对无码影片处理
  always_imagecut: false            # 总是执行裁剪
  aspect_ratio: 2.12                # 图片宽高比
```

#### 水印处理
```yaml
watermark:
  switch: true                      # 启用水印
  water: 2                         # 水印位置 (1-4)
```

#### 额外封面图
```yaml
extrafanart:
  switch: true                      # 下载额外封面图
  extrafanart_folder: "extrafanart" # 额外封面图文件夹
  parallel_download: 1              # 并行下载数
```

### 4. 翻译功能

```yaml
translate:
  switch: true                      # 启用翻译
  engine: "google-free"             # 翻译引擎
  target_language: "zh_cn"          # 目标语言
  values: "title,outline"           # 翻译字段
  delay: 1                          # 翻译延迟
```

### 5. 性能优化

#### 多线程处理
```yaml
common:
  multi_threading: 4                # 并发数 (0=顺序处理)
```

#### 缓存控制
```yaml
common:
  mapping_table_validity: 7         # 映射表有效期(天)
  download_only_missing_images: true # 仅下载缺失图片
```

---

## 命令行参数

### 基本参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-config` | 指定配置文件路径 | `-config /path/to/config.yaml` |
| `-file` | 处理单个文件 | `-file "SSIS-001.mp4"` |
| `-path` | 处理指定目录 | `-path "/movies"` |
| `-mode` | 覆盖配置中的运行模式 | `-mode 1` |
| `-number` | 指定番号(覆盖自动识别) | `-number "SSIS-001"` |

### 数据源参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-search` | 仅搜索指定番号 | `-search "SSIS-001"` |
| `-source` | 指定数据源 | `-source "javbus"` |
| `-url` | 指定具体URL | `-url "https://..."` |

### 调试参数

| 参数 | 说明 | 示例 |
|------|------|------|
| `-debug` | 启用调试模式 | `-debug` |
| `-logdir` | 指定日志目录 | `-logdir "./logs"` |
| `-version` | 显示版本信息 | `-version` |

### 使用示例

```bash
# 基本使用
./mdc -file "SSIS-001.mp4"

# 使用指定配置文件
./mdc -config custom.yaml -path "/movies"

# 强制指定番号
./mdc -file "movie.mp4" -number "SSIS-001"

# 仅搜索不处理
./mdc -search "SSIS-001" -source "javbus"

# 调试模式处理
./mdc -debug -file "SSIS-001.mp4" -logdir "./debug_logs"

# 覆盖运行模式
./mdc -mode 3 -path "/movies"  # 分析模式批量处理
```

---

## 故障排除

### 常见错误及解决方案

#### 1. 网络连接问题

**现象**:
```
Error: Failed to fetch data from website
Error: Connection timeout
```

**解决方案**:
1. 检查网络连接
2. 配置代理设置：
   ```yaml
   proxy:
     switch: true
     proxy: "127.0.0.1:10808"
     type: "socks5"
   ```
3. 增加超时时间：
   ```yaml
   proxy:
     timeout: 60
     retry: 10
   ```

#### 2. 番号识别失败

**现象**:
```
Error: Cannot extract number from filename
Error: No data found for number
```

**解决方案**:
1. 检查文件名格式
2. 使用`-number`参数手动指定：
   ```bash
   ./mdc -file "movie.mp4" -number "SSIS-001"
   ```
3. 检查自定义正则表达式：
   ```yaml
   name_rule:
     number_regexs: "custom_pattern"
   ```

#### 3. 权限问题

**现象**:
```
Error: Permission denied
Error: Failed to create directory
```

**解决方案**:
1. 检查文件夹权限
2. 使用管理员权限运行（Windows）
3. 修改文件夹权限（Linux/macOS）：
   ```bash
   chmod 755 /path/to/folder
   ```

#### 4. 图片下载失败

**现象**:
```
Error: Failed to download image
Error: Invalid image URL
```

**解决方案**:
1. 检查网络连接和代理设置
2. 启用仅下载缺失图片：
   ```yaml
   common:
     download_only_missing_images: true
   ```
3. 检查输出目录权限

### 调试技巧

#### 1. 启用详细日志

```bash
./mdc -debug -logdir "./logs" -file "test.mp4"
```

#### 2. 测试网络连接

```bash
./mdc -search "SSIS-001" -source "javbus" -debug
```

#### 3. 验证配置文件

```bash
./mdc -config config.yaml -version
```

#### 4. 检查分片识别

运行分片处理的单元测试：
```bash
go test ./pkg/fragment/ -v
```

---

## 最佳实践

### 1. 目录结构规划

**推荐结构**:
```
/media/
├── raw_movies/           # 原始文件
├── organized/            # 整理后的文件
│   ├── JAV/             # 有码作品
│   └── Uncensored/      # 无码作品
├── failed/              # 处理失败的文件
└── config/              # 配置文件目录
```

**对应配置**:
```yaml
common:
  source_folder: "/media/raw_movies"
  success_output_folder: "/media/organized/JAV"
  failed_output_folder: "/media/failed"
```

### 2. 代理配置建议

**推荐代理软件**:
- Clash
- V2Ray
- Shadowsocks

**代理配置模板**:
```yaml
proxy:
  switch: true
  proxy: "127.0.0.1:10808"          # Clash默认端口
  type: "socks5"
  timeout: 30
  retry: 5
```

### 3. 批量处理策略

**小批量处理** (推荐):
```bash
# 分批处理，便于监控和错误处理
./mdc -path "/movies/batch1" -debug
./mdc -path "/movies/batch2" -debug
```

**多线程配置**:
```yaml
common:
  multi_threading: 2                # 建议不超过4
```

### 4. 媒体中心集成

#### Kodi配置
1. 启用NFO和图片：
   ```yaml
   # NFO标准格式
   nfo: true
   extrafanart:
     switch: true
   ```

2. 目录结构优化：
   ```yaml
   name_rule:
     location_rule: "actor + '/' + number"
     naming_rule: "number + ' - ' + title"
   ```

#### Jellyfin/Emby配置
1. 使用软链接模式：
   ```yaml
   common:
     link_mode: 1
   ```

### 5. 存储优化

**使用软链接节省空间**:
```yaml
common:
  link_mode: 1                      # 软链接模式
```

**定期清理**:
```yaml
common:
  del_empty_folder: true            # 删除空文件夹
  failed_move: true                 # 移动失败文件
```

---

## 常见问题

### Q1: 程序无法识别番号怎么办？

**A**: 
1. 检查文件名是否包含清晰的番号格式
2. 使用`-number`参数手动指定番号
3. 调整番号提取的正则表达式配置
4. 查看调试日志了解识别过程

### Q2: 下载速度很慢或经常失败？

**A**:
1. 配置稳定的代理服务器
2. 增加重试次数和超时时间
3. 启用"仅下载缺失图片"选项
4. 检查网络连接稳定性

### Q3: 生成的NFO文件在媒体中心中不显示？

**A**:
1. 确认NFO文件名与视频文件名一致
2. 检查媒体中心的刮削器设置
3. 验证NFO文件格式是否正确
4. 确认媒体中心支持的NFO格式

### Q4: 分片文件没有被正确识别？

**A**:
1. 检查分片文件命名格式
2. 运行分片测试工具验证
3. 查看支持的分片格式列表
4. 手动重命名文件以符合支持格式

### Q5: 软链接在Windows下不工作？

**A**:
1. 确保以管理员权限运行
2. 检查Windows版本是否支持软链接
3. 考虑使用硬链接模式
4. 验证目标文件系统支持链接

### Q6: 如何批量处理大量文件？

**A**:
1. 建议分批处理，每批不超过100个文件
2. 使用适当的多线程设置（2-4线程）
3. 监控系统资源使用情况
4. 使用调试模式跟踪处理进度

### Q7: 配置文件被重置了？

**A**:
1. 备份自定义配置文件
2. 检查配置文件权限
3. 使用`-config`参数指定配置文件位置
4. 验证YAML语法格式正确性

---

## 技术支持

### 获取帮助

- 📖 **文档**: 查看完整的项目文档
- 🐛 **问题反馈**: [GitHub Issues](https://github.com/Feng4/movie_data_capture_go/issues)
- 💬 **讨论**: [GitHub Discussions](https://github.com/Feng4/movie_data_capture_go/discussions)

### 日志和调试

**生成调试日志**:
```bash
./mdc -debug -logdir "./logs" -file "test.mp4"
```

**检查配置**:
```bash
./mdc -config config.yaml -version
```

**测试网络连接**:
```bash
./mdc -search "TEST-001" -debug
```

---

## 版本历史

### v1.1.0 (2026-08-01)
- ✨ 新增 **麻豆区（madouqu）** 国产传媒数据源，已加入默认源列表
- 🧹 清理仓库：移除误提交的编译产物与根目录临时调试脚本，新增 `.gitignore`
- 🐛 修复工作池 `Stop()` 死锁（未取消 context 时会永久阻塞）
- 🐛 修复状态恢复模块自死锁与零值配置陷阱
- 🐛 修复 `GetStats()`/`GetMetrics()` 拷贝含锁结构体的并发缺陷
- 📝 修正文档中失实的数据源数量、测试覆盖率与失效的命令引用

### v1.0.0 (2025-08-30)
- ✨ 首个正式版本发布
- 🚀 支持多站点数据抓取
- 🎯 智能分片文件处理
- 📊 NFO文件生成
- 🖼️ 图片下载和处理
- 🌐 代理支持

---

**感谢使用 Movie Data Capture Go！**

如果这个工具对你有帮助，请考虑给项目一个 ⭐ Star！