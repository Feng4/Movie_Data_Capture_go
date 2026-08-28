# Windows 配置文件详细说明

本文档针对 Windows 用户，对 `config.yaml` 中需要特别注意的配置项进行说明。

## 1. 路径格式（重要）

Windows 下推荐使用**正斜杠 `/`** 作为路径分隔符，以兼容跨平台并避免 YAML 转义问题。

| 场景 | 推荐写法 | 错误写法 | 说明 |
|------|----------|----------|------|
| 本地盘符 | `D:/movies` | `D:\movies` | 单反斜杠在 YAML 中会被转义 |
| 相对路径 | `./movies` | `.\\movies` | 正斜杠最安全 |
| UNC 网络 | `\\\\server\\share` | `\\server\share` | YAML 中 `\` 需双写，故 UNC 需四反斜杠 |
| 驱动器根 | `C:/` | `C:\` | 推荐正斜杠 |

### 示例
```yaml
common:
  source_folder: "D:/movies"
  # source_folder: "D:\\movies"       # 也可：双反斜杠
  # source_folder: "D:\movies"        # 错误：单反斜杠

strm:
  network_base_path: "\\\\192.168.1.100\\movies"
```

## 2. 文件链接模式 (link_mode)

`common.link_mode` 在 Windows 下有特殊要求：

| 值 | 模式 | Windows 要求 | 说明 |
|----|------|--------------|------|
| 0 | 移动 | 无 | 默认，跨磁盘支持 |
| 1 | 硬链接 | 同一 NTFS 盘 | 跨盘会失败 |
| 2 | 软链接 | 管理员/开发者模式 | Win10/11 需启用开发者模式或管理员运行 |

**建议**：新手使用 `0` 或 `1`。软链接失败时，请以管理员身份运行或启用开发者模式。

## 3. 代理配置 (proxy)

常见代理软件默认端口：

| 软件 | SOCKS5 端口 | HTTP 端口 | 示例 |
|------|-------------|-----------|------|
| Clash | 7890 | 7891 | `proxy: "127.0.0.1:7890"`, `type: "socks5"` |
| V2RayN | 10808 | 10809 | `proxy: "127.0.0.1:10808"`, `type: "socks5"` |
| Shadowsocks | 1080 | - | `proxy: "127.0.0.1:1080"`, `type: "socks5"` |

## 4. 命名规则中的分隔符

`name_rule.location_rule` 中的路径分隔符**必须用 `/`**，即使在 Windows 下：
```yaml
name_rule:
  location_rule: "actor + '/' + number"  # 正确
  # location_rule: "actor + '\\' + number"  # 错误
```

## 5. 完整 Windows 配置示例

```yaml
common:
  main_mode: 1
  source_folder: "D:/movies"
  link_mode: 1
  sleep: 3

proxy:
  switch: true
  proxy: "127.0.0.1:10808"
  type: "socks5"

name_rule:
  location_rule: "actor + '/' + number"
  naming_rule: "number + '-' + title"
```

## 6. 常见问题

1. **软链接失败**：以管理员运行，或启用「设置 → 系统 → 开发者选项 → 开发者模式」。
2. **路径无效**：检查是否误用单反斜杠，改为 `/` 或 `\\`。
3. **UNC 路径无效**：YAML 中需写四个反斜杠 `\\\\`，或改用 `smb://` 格式。
