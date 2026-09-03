# 快速开始

五步跑通完整流程。前置条件：已按 [安装指南](Installation.md) 完成。

## 第 1 步: 初始化

```bash
./mdc -version    # 确认程序可运行
./mdc             # 首次运行，自动在当前目录生成默认 config.yaml
```

## 第 2 步: 修改配置

用任意编辑器打开 `config.yaml`，首次至少确认以下两项：

```yaml
common:
  main_mode: 1                  # 刮削模式（默认即可）
  source_folder: "./movies"     # 改成你的电影目录
  link_mode: 0                  # 0=移动文件

proxy:
  switch: true                  # 国内网络强烈建议开启
  proxy: "127.0.0.1:10808"      # 改成你的代理地址
  type: "socks5"
```

> 不确定代理端口？常见：Clash 7890（http）/1080，V2Ray 10808（socks5）。也可暂时 `switch: false` 直连试运行。

## 第 3 步: 单文件测试

```bash
./mdc -file "SSIS-001.mp4"
```

成功标志：`JAV_output/演员名/SSIS-001/` 目录下出现视频、`.nfo`、`poster.jpg`、`fanart.jpg` 等文件。

```bash
# 文件名识别不出番号时，手动指定
./mdc -file "神秘视频.mp4" -number "SSIS-001"

# 只想先查查数据源能查到什么
./mdc -search "SSIS-001"
```

## 第 4 步: 批量处理

```bash
./mdc -path "/path/to/movies"
```

程序会扫描目录（含子目录）中所有支持格式的视频文件，逐个刮削整理。失败的文件移入 `failed/`。

> 小于 120MB 的视频默认视为广告跳过；分片文件（如 `-cd1/-cd2`）自动分组处理，详见 [分片文件处理](Fragment-Handling.md)。

## 第 5 步: 接入媒体中心

将 `JAV_output` 添加为 Kodi / Jellyfin / Emby 的媒体库目录，刮削器选择本地 NFO 即可读取元数据。

## 接下来

- [配置详解](Configuration.md): 调整命名规则、水印、人脸裁剪等
- [运行模式](Running-Modes.md): 了解三种模式与软/硬链接用法
- [数据源列表](Data-Sources.md): 启用更多数据源
