# 配置详解

`config.yaml` 全参数参考。查找顺序：`-config` 指定路径 → `./config.yaml` → `./config.yml` → `~/mdc.yaml` → `~/.mdc.yaml` 等；均不存在时自动生成默认配置。带注释模板见 [config_template.yaml](../../config_template.yaml)。

**约定**: 下表「默认值」为程序内置默认（未配置时生效）；「取值」列出程序校验或实际支持的值；「用途」说明实际行为。

## 目录

- [common — 通用](#common--通用)
- [proxy — 代理](#proxy--代理)
- [name_rule — 命名规则](#name_rule--命名规则)
- [update — 更新检查](#update--更新检查)
- [priority — 数据源优先级](#priority--数据源优先级)
- [escape — 转义](#escape--转义)
- [debug_mode — 调试](#debug_mode--调试)
- [translate — 翻译](#translate--翻译)
- [trailer — 预告片](#trailer--预告片)
- [uncensored — 无码识别](#uncensored--无码识别)
- [media — 媒体类型](#media--媒体类型)
- [watermark — 水印](#watermark--水印)
- [extrafanart — 剧照](#extrafanart--剧照)
- [storyline — 剧情简介](#storyline--剧情简介)
- [cc_convert — 中文转换](#cc_convert--中文转换)
- [javdb — JavDB 站点](#javdb--javdb-站点)
- [face — 人脸识别](#face--人脸识别)
- [jellyfin — Jellyfin 适配](#jellyfin--jellyfin-适配)
- [actor_photo — 演员头像](#actor_photo--演员头像)

---

## common — 通用

```yaml
common:
  main_mode: 1
  source_folder: "./"
  success_output_folder: "JAV_output"
  failed_output_folder: "failed"
  link_mode: 0
  scan_hardlink: false
  failed_move: true
  auto_exit: false
  translate_to_sc: true
  actor_gender: "female"
  del_empty_folder: true
  nfo_skip_days: 30
  ignore_failed_list: false
  download_only_missing_images: true
  mapping_table_validity: 7
  jellyfin: 0
  actor_only_tag: false
  sleep: 3
  anonymous_fill: 0
  multi_threading: 0
  stop_counter: 0
  rerun_delay: "0"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `main_mode` | int | `1` | 0~3 | 运行模式。1=刮削（完整流程），2=整理（不联网），3=分析（原地生成 NFO 不动文件）。详见[运行模式](Running-Modes.md) |
| `source_folder` | string | `"./"` | 任意路径 | 未用 `-path` 时的扫描目录；目录不存在会校验失败 |
| `success_output_folder` | string | `"JAV_output"` | 任意路径 | 刮削成功后的输出目录（模式 1/2） |
| `failed_output_folder` | string | `"failed"` | 任意路径 | 刮削失败文件的归档目录 |
| `link_mode` | int | `0` | 0~2 | 归档方式：0=移动，1=软链接，2=硬链接（失败回退软链） |
| `scan_hardlink` | bool | `false` | true/false | 扫描时是否进入/处理硬链接文件 |
| `failed_move` | bool | `true` | true/false | 失败文件是否移入 `failed_output_folder` |
| `auto_exit` | bool | `false` | true/false | 处理完成后是否自动退出 |
| `translate_to_sc` | bool | `true` | true/false | 繁体元数据转简体 |
| `actor_gender` | string | `"female"` | `female` / `male` / `both` / 留空 | 演员性别过滤，影响 NFO 中的演员信息 |
| `del_empty_folder` | bool | `true` | true/false | 处理后删除源目录及输出目录中的空文件夹 |
| `nfo_skip_days` | int | `30` | ≥0（天） | 模式 3 下，NFO 在 N 天内更新过的文件直接跳过，避免重复刮削 |
| `ignore_failed_list` | bool | `false` | true/false | 默认跳过失败列表中的文件；置 true 则忽略列表强制重试 |
| `download_only_missing_images` | bool | `true` | true/false | 只下载本地缺失的图片，重跑时不重复下载 |
| `mapping_table_validity` | int | `7` | ≥1（天） | 番号映射表缓存有效期，期内重复运行不发请求 |
| `jellyfin` | int | `0` | 0 / >0 | 0=通用模式（同时生成 fanart 副本）；>0=Jellyfin 兼容模式 |
| `actor_only_tag` | bool | `false` | true/false | NFO 标签只使用演员姓名 |
| `sleep` | int | `3` | 0~60（秒，建议） | 每个文件处理前的间隔，防请求过快被封 |
| `anonymous_fill` | int | `0` | 0 / >0 | 未知演员的填充方式（如统一匿名标签） |
| `multi_threading` | int | `0` | 0~20（建议） | 并发数，0=顺序处理。建议 2~4 |
| `stop_counter` | int | `0` | ≥0 | 处理 N 个后停止，0=不限制。适合分批跑大批量 |
| `rerun_delay` | string | `"0"` | 秒数或 `1h30m45s` 格式 | 完整一轮后的重跑延迟 |

## proxy — 代理

```yaml
proxy:
  switch: true
  proxy: "127.0.0.1:10808"
  timeout: 30
  retry: 5
  type: "socks5"
  cacert_file: ""
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `switch` | bool | `true` | true/false | 是否启用代理。国内网络强烈建议开启 |
| `proxy` | string | — | `host:port` | 代理地址。开启 switch 后必填，否则校验失败 |
| `timeout` | int | `30` | >0，建议 ≤300（秒） | 单次请求超时时间 |
| `retry` | int | `5` | ≥0，建议 ≤10 | 网络失败重试次数 |
| `type` | string | `"socks5"` | `http` / `https` / `socks5` / `socks4` | 代理协议类型 |
| `cacert_file` | string | `""` | 文件路径 | 自定义 CA 证书（抓 HTTPS 站点证书异常时使用） |

常见代理端口参考：Clash `7890`(http) / `1080`(socks5)，V2Ray `10808`(socks5)。

## name_rule — 命名规则

```yaml
name_rule:
  location_rule: "actor + '/' + number"
  naming_rule: "number + '-' + title"
  max_title_len: 50
  image_naming_with_number: false
  number_uppercase: false
  number_regexs: ""
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `location_rule` | string | `"actor + '/' + number"` | 变量拼接表达式 | 输出目录层级规则。可用变量：`actor`、`number`、`title`、`year`、`studio`、`label`。**必须含 `number`**，否则校验失败 |
| `naming_rule` | string | `"number + '-' + title"` | 变量拼接表达式 | 视频文件命名规则，变量同上，**必须含 `number`** |
| `max_title_len` | int | `50` | >0 | 文件名中标题最大长度，超出截断 |
| `image_naming_with_number` | bool | `false` | true/false | true 时图片名带番号前缀（如 `SSIS-001-poster.jpg`），false 用通用名（`poster.jpg`） |
| `number_uppercase` | bool | `false` | true/false | 番号统一转大写 |
| `number_regexs` | string | `""` | 逗号分隔的正则 | 自定义番号提取正则，弥补内置规则。语法错误会在校验时报错 |

命名效果示例见[命名与输出](Naming-and-Output.md)。

## update — 更新检查

```yaml
update:
  update_check: true
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `update_check` | bool | `true` | true/false | 启动时检查新版本 |

## priority — 数据源优先级

```yaml
priority:
  website: "javbus,fanza,fc2,fc2club,javdb,xcity,mgstage,jav321,madouqu"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `website` | string | 见左 | 逗号分隔的数据源配置名 | 按顺序依次尝试，直到成功。全部 24 个配置名见[数据源列表](Data-Sources.md) |

调优建议：把命中率高、速度快的源放前面；国产源（`madouqu` 等）放末位不影响日系番号速度。

## escape — 转义

```yaml
escape:
  literals: "\\()/ "
  folders: "failed, JAV_output"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `literals` | string | `"\\()/ "` | 任意字符集 | 文件名中出现这些字符时替换处理，避免路径非法 |
| `folders` | string | `"failed, JAV_output"` | 逗号分隔目录名 | 扫描时跳过这些目录（防止把输出目录再扫一遍） |

## debug_mode — 调试

```yaml
debug_mode:
  switch: false
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `switch` | bool | `false` | true/false | 调试模式：输出 DEBUG 级日志、刮削数据全文打印，且小于 120MB 的视频不再被当广告跳过。等效命令行 `-debug` |

## translate — 翻译

```yaml
translate:
  switch: false
  engine: "google-free"
  target_language: "zh_cn"
  key: ""
  delay: 1
  values: "title,outline"
  service_site: "translate.google.cn"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `switch` | bool | `false` | true/false | 启用翻译 |
| `engine` | string | `"google-free"` | `google-free` / `google` / `baidu` / `youdao` / `deepl` | 翻译引擎。`google-free` 免密钥；其他需配合 `key` |
| `target_language` | string | `"zh_cn"` | `zh_cn` / `zh_tw` / `en` / `ja` / `ko` / `fr` / `de` / `es` / `ru` | 目标语言 |
| `key` | string | `""` | API 密钥 | 付费/密钥引擎的密钥 |
| `delay` | int | `1` | 0~30（秒，建议） | 翻译请求间隔 |
| `values` | string | `"title,outline"` | `title` / `outline` / `tag` / `series` / `studio` / `director` / `actor` 逗号组合 | 需要翻译的字段 |
| `service_site` | string | `"translate.google.cn"` | 域名 | 翻译服务站点地址 |

## trailer — 预告片

```yaml
trailer:
  switch: false
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `switch` | bool | `false` | true/false | 下载官方预告片，保存为 `番号-trailer.mp4` |

## uncensored — 无码识别

```yaml
uncensored:
  uncensored_prefix: "S2M,BT,LAF,SMD"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `uncensored_prefix` | string | `"S2M,BT,LAF,SMD"` | 逗号分隔前缀 | 番号以这些前缀开头判定为无码作品，影响水印角标、人脸裁剪范围与归档逻辑 |

## media — 媒体类型

```yaml
media:
  media_type: ".mp4,.avi,.rmvb,.wmv,.mov,.mkv,.flv,.ts,.webm,.iso"
  sub_type: ".smi,.srt,.idx,.sub,.sup,.psb,.ssa,.ass,.usf,.xss,.ssf,.rt,.lrc,.sbv,.vtt,.ttml"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `media_type` | string | 见左 | 逗号分隔扩展名，**每项以 `.` 开头** | 参与刮削的视频扩展名白名单 |
| `sub_type` | string | 见左 | 同上 | 字幕扩展名白名单 |

> 扫描时小于 120MB 的视频文件默认视为广告跳过（`-debug` 可强制处理）。

## watermark — 水印

```yaml
watermark:
  switch: true
  water: 2
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `switch` | bool | `true` | true/false | 为 poster/thumb 添加属性角标（中字 `-C`、流出 `-leak`、破解 `-hack`、4K、无码等），素材位于 `Img/` 目录 |
| `water` | int | `2` | 1~4 | 角标位置：1=左上，2=右上，3=左下，4=右下 |

## extrafanart — 剧照

```yaml
extrafanart:
  switch: true
  extrafanart_folder: "extrafanart"
  parallel_download: 1
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `switch` | bool | `true` | true/false | 下载额外剧照，存入影片目录的子文件夹 |
| `extrafanart_folder` | string | `"extrafanart"` | 目录名 | 剧照子文件夹名称 |
| `parallel_download` | int | `1` | ≥1，建议 ≤10 | 剧照并行下载数 |

## storyline — 剧情简介

```yaml
storyline:
  switch: true
  site: "1:avno1"
  censored_site: "5:xcity,6:amazon"
  uncensored_site: "3:58avgo"
  show_result: 0
  run_mode: 1
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `switch` | bool | `true` | true/false | 启用剧情简介抓取 |
| `site` | string | `"1:avno1"` | `编号:站点` | 通用简介来源 |
| `censored_site` | string | `"5:xcity,6:amazon"` | 逗号组合 | 有码影片的简介来源（按序尝试） |
| `uncensored_site` | string | `"3:58avgo"` | 逗号组合 | 无码影片的简介来源 |
| `show_result` | int | `0` | 0/1 | 是否在日志中打印简介抓取结果 |
| `run_mode` | int | `1` | 0/1 | 简介抓取运行模式（1=抓不到不阻塞主流程） |

## cc_convert — 中文转换

```yaml
cc_convert:
  mode: 1
  vars: "actor,director,label,outline,series,studio,tag,title"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `mode` | int | `1` | 0/1 | 简繁转换模式（0=关闭，1=开启） |
| `vars` | string | 见左 | 逗号分隔字段 | 参与简繁转换的元数据字段 |

## javdb — JavDB 站点

```yaml
javdb:
  sites: "38,39"
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `sites` | string | `"38,39"` | 逗号分隔站点后缀 | JavDB 镜像站点编号，抓取失败时可更换 |

## face — 人脸识别

```yaml
face:
  locations_model: "hog"
  uncensored_only: true
  always_imagecut: false
  aspect_ratio: 2.12
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `locations_model` | string | `"hog"` | `hog` / `cnn` / 留空 | 人脸检测模型。hog 快、通用；cnn 更准更慢 |
| `uncensored_only` | bool | `true` | true/false | 仅对无码影片执行人脸裁剪 |
| `always_imagecut` | bool | `false` | true/false | 无需人脸检测也强制裁剪出 poster |
| `aspect_ratio` | float | `2.12` | >0，建议 ≤10 | 裁剪海报的宽高比 |

> FC2 番号自动跳过裁剪（封面三图同源，直接复制）。

## jellyfin — Jellyfin 适配

```yaml
jellyfin:
  multi_part_fanart: false
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `multi_part_fanart` | bool | `false` | true/false | 多分片影片是否合并 fanart |

## actor_photo — 演员头像

```yaml
actor_photo:
  download_for_kodi: false
```

| 参数 | 类型 | 默认值 | 取值 | 用途 |
|------|------|--------|------|------|
| `download_for_kodi` | bool | `false` | true/false | 下载演员头像供 Kodi 展示 |

## 配置校验

程序启动时自动校验配置，常见报错：

| 报错 | 原因与修复 |
|------|-----------|
| `invalid link_mode: N, must be 0-2` | link_mode 超出范围 |
| `source folder does not exist` | source_folder 路径不存在 |
| `proxy is enabled but proxy URL is empty` | switch 开了但 proxy 为空 |
| `invalid proxy type: xxx` | type 不在 http/https/socks5/socks4 内 |
| `location_rule must contain 'number' variable` | 命名规则缺 number 变量 |
| `invalid number regex 'xxx'` | number_regexs 正则语法错误 |
| `invalid translate engine / target language` | translate 参数不在枚举内 |
| `max_title_len must be positive` | max_title_len ≤ 0 |
| `media type must start with dot` | media_type 某项缺前导 `.` |

## 下一步

- [运行模式](Running-Modes.md)
- [命名与输出](Naming-and-Output.md)
