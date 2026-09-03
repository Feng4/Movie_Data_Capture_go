# Movie Data Capture Go Wiki

**MDC-Go** 是一个用 Go 编写的电影元数据自动刮削和整理工具：识别番号 → 多站点抓取数据 → 生成 NFO → 下载处理封面海报 → 按规则整理媒体库。单一可执行文件、跨平台、无运行时依赖。

> 本项目是 [mvdctop/Movie_Data_Capture](https://github.com/mvdctop/Movie_Data_Capture)（Python 版）的 Go 重写版。

## 功能一览

| 类别 | 功能 |
|------|------|
| 刮削 | 24 个数据源（JavBus / FANZA / JavDB / 麻豆区等），按优先级自动切换 |
| 识别 | 智能番号解析，支持自定义正则；分片文件自动分组 |
| 元数据 | Kodi / Jellyfin / Emby 兼容的 NFO 生成 |
| 图片 | 封面、海报、剧照下载；人脸识别裁剪；属性角标水印 |
| 整理 | 自定义目录与命名规则；移动 / 软链接 / 硬链接三种归档方式 |
| 网络 | HTTP / SOCKS5 代理、超时重试、多线程、限速防封 |
| 容错 | 失败目录管理、失败列表跳过、映射表缓存、断点续跑 |

## 章节导航

| 页面 | 内容 |
|------|------|
| [安装指南](Installation.md) | 预编译包 / 源码编译 / Docker 三种安装方式 |
| [快速开始](Quick-Start.md) | 五分钟跑通第一个文件 |
| [命令行参数](CLI-Reference.md) | 全部启动参数与常用命令组合 |
| [配置详解](Configuration.md) | config.yaml 全参数说明：取值、默认值、用途 |
| [运行模式](Running-Modes.md) | 刮削 / 整理 / 分析三种模式，移动 / 软链 / 硬链三种归档 |
| [数据源列表](Data-Sources.md) | 24 个数据源的启用方法与优先级调优 |
| [命名与输出](Naming-and-Output.md) | 目录结构规则、文件名变量、输出目录、水印角标 |
| [分片文件处理](Fragment-Handling.md) | CD / Part / Disc 等分片格式的识别与归档 |
| [故障排除](Troubleshooting.md) | 常见问题速查表与 FAQ |

## 极简上手

```bash
./mdc -version            # 确认可运行
./mdc                     # 首次运行生成默认 config.yaml
# 编辑 config.yaml（源目录 + 代理）
./mdc -file "SSIS-001.mp4"   # 测试单个文件
./mdc -path "/movies"        # 批量处理
```

详细步骤见 [快速开始](Quick-Start.md)。

## 相关资源

- [项目 README](../../README.md)
- [快速入门指南](../../QUICK_START.md)
- [用户手册](../../USER_MANUAL.md)
- [问题反馈](https://github.com/Feng4/movie_data_capture_go/issues)
