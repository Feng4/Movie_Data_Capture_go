# 故障排除

常见问题速查与 FAQ。

## 速查表

| 现象 | 原因 | 处理方法 |
|------|------|---------|
| 连接超时 / 全部刮削失败 | 代理未开或配置错误 | 检查 `proxy.switch`、`proxy.proxy` 地址端口、`proxy.type`；Docker 下填宿主机 IP 而非 127.0.0.1 |
| 某站点持续失败 | 站点被墙/改版 | 调整 `priority.website` 顺序，把可用源前置；JavDB 可换 `javdb.sites` 镜像 |
| `Cannot extract number from filename` | 文件名无规范番号 | 用 `-number` 手动指定；或配置 `name_rule.number_regexs` 自定义正则 |
| 文件直接被跳过不处理 | 体积 < 120MB 被判为广告 | 加 `-debug` 强制处理；或确认文件完整 |
| NFO 在媒体中心不显示 | NFO 与视频不同名/不同目录 | 模式 3 下 NFO 会与视频同名；检查媒体中心刮削器是否启用本地 NFO |
| 分片没被分到一组 | 命名不规范 | 按标准格式改名（`-cd1`、`-part_1` 等），详见[分片处理](Fragment-Handling.md) |
| Windows 软链接报错 | 无管理员权限 | 以管理员运行；或改 `link_mode: 2`（硬链接）；或开启系统开发者模式 |
| 图片下载不完整 | 网络波动 | 保持 `download_only_missing_images: true` 重跑，只补缺失 |
| 启动报配置校验错误 | 参数取值非法 | 对照[配置校验表](Configuration.md#配置校验)修复 |
| 重跑又刮了一遍 | 映射表过期/失败列表 | `mapping_table_validity` 调大；模式 3 配合 `nfo_skip_days` |
| 日志太多看不过来 | DEBUG 刷屏 | 关闭 `debug_mode`（默认只输出 INFO 及以上）；日志落盘用 `-logdir` |

## FAQ

### Q1: 刮削失败的文件去哪了？

移入 `failed_output_folder`（默认 `failed/`）。默认情况下重跑会跳过失败列表中的文件；想强制重试设 `ignore_failed_list: true` 或加 `-debug`。

### Q2: 怎么给现有媒体库补 NFO 而不动文件？

```bash
./mdc -mode 3 -path "/media/library"
```

分析模式原地生成 NFO 与图片，配合 `nfo_skip_days: 30` 避免重复处理。

### Q3: 番号识别不准怎么办？

1. 优先规范文件名（番号尽量前置、用 `-` 分隔）
2. 特殊番号（FC2、无前缀等）内置规则已覆盖，用 `-search` 先验证
3. 仍失败时用 `-number` 手动指定，或 `number_regexs` 写自定义正则，示例：

```yaml
name_rule:
  number_regexs: "([A-Z]{2,5})-?(\\d{3})"
```

### Q4: 大批量文件怎么跑比较稳？

```yaml
common:
  multi_threading: 4      # 并发 2~4
  sleep: 1                # 降低间隔
  stop_counter: 200       # 分批，每批 200 个
```

配合软/硬件资源观察日志中 `Processing completed: N successful, M failed` 汇总。

### Q5: 代理类型怎么选？

| 场景 | type |
|------|------|
| Clash 混合端口 | `http`（7890） |
| V2Ray socks | `socks5`（10808） |
| 远端 DNS 解析 | `socks5`（校验也接受 `socks5h`/`socks4`/`https`） |

### Q6: 水印角标没打上？

- 确认 `watermark.switch: true`
- 确认程序目录含 `Img/` 素材文件夹
- 角标依据文件名属性触发（`-C`、`-leak` 等），详见[命名与输出](Naming-and-Output.md)

### Q7: 人脸裁剪把海报裁歪了？

调 `face.aspect_ratio`（默认 2.12）适配封面构图；或 `face.uncensored_only: true` 缩小作用范围；`locations_model: "cnn"` 换更准的模型。FC2 内容自动跳过裁剪。

### Q8: Windows 下 PowerShell 执行提示风险？

首次运行如有 SmartScreen 拦截，右键文件属性解除锁定；源码编译则无此问题。

### Q9: 配置文件每次都变回默认？

确认编辑的是程序实际加载的那份（查找顺序见[配置详解](Configuration.md)）。用 `-config` 显式指定路径最稳妥。

### Q10: 如何确认数据源能查到某番号？

```bash
./mdc -search "SSIS-001"               # 按优先级全链路搜
./mdc -search "SSIS-001" -source javbus # 只试 JavBus
```

返回元数据即链路通畅；返回 `No data found` 则换源或检查番号拼写。

## 仍然解决不了？

- 收集调试日志: `./mdc -debug -path "..." -logdir "./logs"`
- 到 [GitHub Issues](https://github.com/Feng4/movie_data_capture_go/issues) 提交，附上日志片段与配置（隐去代理地址）

## 相关

- [配置详解](Configuration.md)
- [数据源列表](Data-Sources.md)
- [安装指南](Installation.md)
