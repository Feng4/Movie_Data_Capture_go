# 快速入门指南

## 🚀 5分钟上手 Movie Data Capture Go

### 第一步：下载和安装

1. 从 [Releases](https://github.com/Feng4/movie_data_capture_go/releases) 下载对应版本
2. 解压到任意文件夹
3. 打开终端/命令行，切换到程序目录

### 第二步：首次运行

```bash
# 检查程序是否正常
./mdc --version

# 生成默认配置文件
./mdc
```

### 第三步：基础配置

编辑 `config.yaml` 文件：

```yaml
# 最基础的配置
common:
  main_mode: 1                    # 完整刮削模式
  source_folder: "./movies"       # 你的电影文件夹
  link_mode: 0                    # 移动文件模式

proxy:
  switch: true                    # 启用代理（推荐）
  proxy: "127.0.0.1:10808"       # 你的代理地址
  type: "socks5"
```

### 第四步：测试单个文件

```bash
# 处理一个测试文件
./mdc -file "SSIS-001.mp4"
```

### 第五步：批量处理

```bash
# 处理整个目录
./mdc -path "./movies"
```

## 🎯 常用场景配置

### 场景1：保持原文件不动（推荐新手）

```yaml
common:
  main_mode: 1
  link_mode: 1                    # 软链接模式
strm:
  enable: true                    # 生成STRM文件
```

### 场景2：完整整理媒体库

```yaml
common:
  main_mode: 1
  link_mode: 0                    # 移动文件
name_rule:
  location_rule: "actor + '/' + number"
  naming_rule: "number + '-' + title"
```

### 场景3：仅生成NFO文件

```yaml
common:
  main_mode: 3                    # 分析模式
```

## ⚡ 快速命令参考

```bash
# 查看版本
./mdc --version

# 处理单个文件
./mdc -file "movie.mp4"

# 批量处理
./mdc -path "/path/to/movies"

# 调试模式
./mdc -debug -file "movie.mp4"

# 指定番号
./mdc -file "movie.mp4" -number "SSIS-001"

# 搜索测试
./mdc -search "SSIS-001"
```

## 🔧 出问题了？

### 1. 网络问题
- 确保代理设置正确
- 测试网络连接

### 2. 识别失败
- 检查文件名是否包含番号
- 使用 `-number` 参数手动指定

### 3. 权限问题
- Windows：以管理员身份运行
- Linux/macOS：检查文件夹权限

## 📚 进一步学习

- 📖 [完整用户手册](USER_MANUAL.md)
- 🔗 [STRM功能指南](docs/STRM_GUIDE.md)
- 🐛 [问题反馈](https://github.com/Feng4/movie_data_capture_go/issues)

---

**恭喜！你已经掌握了基本使用方法！** 🎉