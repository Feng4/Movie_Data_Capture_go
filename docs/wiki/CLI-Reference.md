# 命令行参数

所有参数为可选，未提供时从配置文件读取。`-h` / `--help` 可查看参数列表。

## 参数总表

| 参数 | 类型 | 说明 | 示例 |
|------|------|------|------|
| `-config` | 字符串 | 配置文件路径，默认 `config.yaml` | `-config my.yaml` |
| `-file` | 字符串 | 只处理单个文件 | `-file "SSIS-001.mp4"` |
| `-number` | 字符串 | 手动指定番号，覆盖自动识别（须与 `-file` 搭配） | `-number "SSIS-001"` |
| `-mode` | 整数 | 覆盖配置中的运行模式：1=刮削, 2=整理, 3=分析，默认 1 | `-mode 3` |
| `-path` | 字符串 | 批量处理指定目录（覆盖 `source_folder`） | `-path "/movies"` |
| `-search` | 字符串 | 只查询番号信息并打印，不处理文件 | `-search "SSIS-001"` |
| `-source` | 字符串 | 本次运行只用指定数据源（配置名见 [数据源列表](Data-Sources.md)） | `-source javbus` |
| `-url` | 字符串 | 直接指定详情页 URL 刮削 | `-url "https://www.javbus.com/SSIS-001"` |
| `-debug` | 开关 | 启用调试模式，输出 DEBUG 级日志 | `-debug` |
| `-logdir` | 字符串 | 日志写入指定目录（不指定则只输出到控制台） | `-logdir "./logs"` |
| `-version` | 开关 | 打印版本、Go 版本、平台信息后退出 | `-version` |

> `-mode` 只在该值 ≠ 1 时才覆盖配置；`-path`、`-debug` 一旦提供即覆盖。

## 参数优先级

命令行 > 配置文件 > 程序默认值。

例如配置文件 `main_mode: 3`，运行 `./mdc -mode 1 -path /movies`，则本次为刮削模式。

## 常用组合示例

```bash
# ---------- 日常刮削 ----------

# 批量处理目录（最常用）
./mdc -path "/downloads/movies"

# 处理单个文件，番号自动识别
./mdc -file "SSIS-001.mp4"

# 文件名奇怪、识别失败，手动指定番号
./mdc -file "unknown_video.mp4" -number "SSIS-001"

# ---------- 数据源控制 ----------

# 只搜索不落盘（验证某番号在各站点能否查到）
./mdc -search "MDX-0212" -source madouqu

# 已知详情页 URL，直接刮削
./mdc -file "movie.mp4" -url "https://www.javbus.com/SSIS-001"

# ---------- 模式切换 ----------

# 分析模式：原地生成 NFO，不移动文件（给现有媒体库补元数据）
./mdc -mode 3 -path "/media/library"

# 整理模式：不联网，仅按已有数据归类
./mdc -mode 2 -path "/downloads"

# ---------- 调试与日志 ----------

# 调试模式 + 日志落盘，排查识别/网络问题
./mdc -debug -path "/movies" -logdir "./logs"

# 使用另一份配置文件
./mdc -config jellyfin.yaml -path "/media/nas"
```

## 退出行为

- 全部处理完成后打印 `All finished!` 与运行时长
- 刮削失败的文件移入 `failed/`（可用 `common.ignore_failed_list` 忽略失败列表重试）
- Windows 下日志同时会写入文件时，目录不存在会自动创建

## 下一步

- [配置详解](Configuration.md)
- [运行模式](Running-Modes.md)
