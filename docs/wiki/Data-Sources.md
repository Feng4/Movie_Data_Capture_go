# 数据源列表

MDC-Go 内置 24 个数据源。`默认` 列标注是否在默认 `priority.website` 中启用。

## 数据源总表

| 配置名 | 网站 | 类型 | 默认 |
|--------|------|------|------|
| `javbus` | JavBus | 综合 | ✅ |
| `fanza` | FANZA | 官方 | ✅ |
| `fc2` | `fc2club` | FC2 / FC2Club | 素人 | ✅ |
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

> 带别名的配置名（如 `mdq`）在 `priority.website` 与 `-source` 中均可使用。

## 启用与调整

### 修改优先级顺序

程序按 `priority.website` 的顺序依次尝试，直到某源成功返回数据：

```yaml
priority:
  # 默认：日系优先，国产源放末位
  website: "javbus,fanza,fc2,fc2club,javdb,xcity,mgstage,jav321,madouqu"
```

**调优建议**:

- 命中率高的放前面，减少无效请求
- 只刮特定类型时精简列表，如纯无码: `"carib,caribpr,xcity"`
- 国产源放末位不影响日系番号检索速度

### 启用非默认源

把配置名追加进列表即可：

```yaml
priority:
  website: "javbus,javdb,fanza,carib,getchu"
```

### 临时使用单一数据源

不改配置，命令行直接指定：

```bash
./mdc -source madouqu -number "MDX-0212"
./mdc -source carib -file "052621-001.mp4"
```

### 直接指定 URL

已知详情页时跳过搜索环节：

```bash
./mdc -file "movie.mp4" -url "https://www.javbus.com/SSIS-001"
```

## 按内容类型选源

| 内容类型 | 推荐源 |
|---------|--------|
| 日系主流番号 | `javbus`, `javdb`, `fanza`, `jav321` |
| 无码 | `xcity`, `carib`, `caribpr` |
| FC2 素人 | `fc2`, `fc2club` |
| 素人企划 | `mgstage` |
| 国产传媒（麻豆/天美/蜜桃等） | `madouqu`, `madou`, `javday`, `cnmdb` |
| 同人 | `dlsite`, `gcolle` |
| 动漫 | `getchu` |
| 厂牌专属 | `dahlia`, `faleno`, `fantastica` |

## 多源协同机制

1. 番号解析后按优先级逐个请求
2. 单源失败（超时/无数据）自动切换下一源
3. 结合 `proxy.retry` 与 `proxy.timeout` 控制重试节奏
4. 剧情简介独立配置来源（见[配置详解 — storyline](Configuration.md#storyline--剧情简介)）
5. `mapping_table_validity` 缓存番号结果，重复运行不重复请求

## 相关

- [配置详解](Configuration.md)
- [故障排除](Troubleshooting.md)
