# 命名与输出

刮削成功后的目录结构、文件命名规则、图片体系与水印角标说明。

## 输出目录结构

默认配置（`location_rule: "actor + '/' + number"`）下：

```
JAV_output/
├── 演员名/                        # location_rule 第一层
│   ├── SSIS-001/                  # location_rule 第二层
│   │   ├── SSIS-001.mp4           # 视频文件（naming_rule 生成）
│   │   ├── SSIS-001.nfo           # 元数据（Kodi/Jellyfin/Emby 通用）
│   │   ├── fanart.jpg             # 背景图
│   │   ├── poster.jpg             # 海报（含水印角标）
│   │   ├── thumb.jpg              # 缩略图
│   │   └── extrafanart/           # 额外剧照（可开关）
│   │       ├── extrafanart-1.jpg
│   │       └── extrafanart-2.jpg
failed/
└── 刮削失败/无法识别番号的文件
```

模式 3（分析模式）下，NFO 与图片直接生成在视频所在目录，NFO 与视频同名。

## 命名规则变量

`name_rule.location_rule`（目录）与 `name_rule.naming_rule`（文件名）可使用以下变量，以 `+` 拼接、字符串加引号：

| 变量 | 含义 | 示例值 |
|------|------|--------|
| `actor` | 演员名 | 葵つかさ |
| `number` | 番号 | SSIS-001 |
| `title` | 标题 | 美しい人妻の秘密 |
| `year` | 年份 | 2021 |
| `studio` | 制作商 | S1_NO.1_STYLE |
| `label` | 系列 | S1 |

> 两条规则都**必须包含 `number` 变量**，否则配置校验失败。

## 命名示例

```yaml
# 示例 1（默认）
location_rule: "actor + '/' + number"
naming_rule: "number + '-' + title"
# => JAV_output/葵つかさ/SSIS-001/SSIS-001-美しい人妻の秘密.mp4

# 示例 2: 按制作商归档
location_rule: "studio + '/' + actor + '/' + number"
naming_rule: "actor + '_' + number + '_' + title"
# => JAV_output/S1_NO.1_STYLE/葵つかさ/SSIS-001/葵つかさ_SSIS-001_美しい人妻の秘密.mp4

# 示例 3: 极简（番号即一切）
location_rule: "number"
naming_rule: "number"
# => JAV_output/SSIS-001/SSIS-001.mp4
```

**自动清理**:

- 文件名中的 `\/:*?"<>|` 等系统非法字符自动替换
- 标题超过 `max_title_len`（默认 50）自动截断
- `name_rule.number_uppercase: true` 时番号统一大写

## 图片体系

| 文件 | 来源 | 用途 |
|------|------|------|
| `fanart.jpg` | 封面原图 | 媒体中心背景图（Jellyfin 模式不重复生成） |
| `poster.jpg` | 封面裁剪/小图 | 海报，纵向构图，优先人脸 |
| `thumb.jpg` | 封面原图 | 缩略图 |

- `image_naming_with_number: true` 时改名为 `番号-fanart.jpg` / `番号-poster.jpg` / `番号-thumb.jpg`
- 人脸裁剪由 [face 配置](Configuration.md#face--人脸识别) 控制，FC2 番号自动跳过（三图同源直接复制）
- 重跑时 `download_only_missing_images: true` 只补缺失图片

## 属性角标与文件名后缀

程序从文件名识别影片属性，体现在**文件名后缀**与**海报水印角标**（素材在 `Img/` 目录）：

| 文件名标志 | 含义 | 角标图 |
|-----------|------|--------|
| `-C` | 内嵌中文字幕 | SUB |
| `-leak` | 流出版 | LEAK |
| `-hack` | 破解版 | HACK |
| 番号以 `uncensored_prefix` 开头 | 无码 | UNCENSORED |
| 文件名含 `4K` / `8K` | 高分辨率 | 4K / 8K |
| 文件名含 `-iso` 或 `.iso` | ISO 镜像 | ISO |
| 文件名含 UMR 前缀 | UMR 系列 | UMR |
| 素人（FC2 等） | 素人属性 | YOUMA |

水印位置由 `watermark.water`（1~4）控制，开关见 `watermark.switch`。

**后缀优先级**: hack > leak > C（中字与 hack/leak 不同时生效）。

示例：

```bash
SSIS-001.mp4            → SSIS-001.mp4
SSIS-001-C.mp4          → SSIS-001-C.mp4        # 中字
SSIS-001-leak.mp4       → SSIS-001-leak.mp4     # 流出
FC2-1234567.mp4         → FC2-1234567.mp4       # 素人，跳过裁剪
```

## 分片文件的输出

分片组归档到同一目录，统一改名 `番号-1`、`番号-2`…，NFO 记录总分片数与体积。详见[分片文件处理](Fragment-Handling.md)。

## 相关

- [配置详解](Configuration.md)
- [命名规则配置](Configuration.md#name_rule--命名规则)
