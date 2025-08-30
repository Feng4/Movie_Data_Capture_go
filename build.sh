#!/bin/bash

# Movie Data Capture Go 跨平台编译脚本
# 使用方法: ./build.sh [版本号]

set -e

# 获取版本号
VERSION=${1:-"dev"}
BUILD_TIME=$(date -u '+%Y-%m-%d_%H:%M:%S')
GIT_COMMIT=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")

# 项目信息
PROJECT_NAME="mdc"
MAIN_FILE="main.go"

# 构建选项
LDFLAGS="-s -w -X main.Version=${VERSION} -X main.BuildTime=${BUILD_TIME} -X main.GitCommit=${GIT_COMMIT}"

# 清理之前的构建
echo "🧹 清理之前的构建文件..."
rm -rf dist/
mkdir -p dist/

# 构建平台列表
PLATFORMS=(
    "windows/amd64"
    "windows/386"
    "windows/arm64"
    "linux/amd64"
    "linux/386"
    "linux/arm64"
    "linux/arm"
    "darwin/amd64"
    "darwin/arm64"
)

echo "🚀 开始构建 ${PROJECT_NAME} ${VERSION}..."
echo "📦 构建平台: ${#PLATFORMS[@]} 个"
echo "⏰ 构建时间: ${BUILD_TIME}"
echo "🔖 Git提交: ${GIT_COMMIT}"
echo ""

# 并行构建函数
build_platform() {
    local platform=$1
    local GOOS=${platform%/*}
    local GOARCH=${platform#*/}
    
    # 确定文件扩展名
    local ext=""
    if [ "$GOOS" = "windows" ]; then
        ext=".exe"
    fi
    
    local output_name="${PROJECT_NAME}-${GOOS}-${GOARCH}${ext}"
    local build_dir="dist/${PROJECT_NAME}-${GOOS}-${GOARCH}"
    
    echo "🔨 构建 ${GOOS}/${GOARCH}..."
    
    # 创建构建目录
    mkdir -p "$build_dir"
    
    # 构建二进制文件
    CGO_ENABLED=0 GOOS=$GOOS GOARCH=$GOARCH go build \
        -ldflags="$LDFLAGS" \
        -o "$build_dir/$output_name" \
        $MAIN_FILE
    
    if [ $? -eq 0 ]; then
        # 复制必要文件
        cp config.yaml "$build_dir/"
        cp README.md "$build_dir/"
        cp -r Img "$build_dir/" 2>/dev/null || true
        
        # 创建压缩包
        cd dist/
        if [ "$GOOS" = "windows" ]; then
            # Windows 使用 zip
            if command -v zip >/dev/null 2>&1; then
                zip -r "${PROJECT_NAME}-${GOOS}-${GOARCH}.zip" "${PROJECT_NAME}-${GOOS}-${GOARCH}/" >/dev/null
            else
                echo "⚠️  zip 命令不存在，跳过创建 Windows 压缩包"
            fi
        else
            # Unix 系统使用 tar.gz
            tar -czf "${PROJECT_NAME}-${GOOS}-${GOARCH}.tar.gz" "${PROJECT_NAME}-${GOOS}-${GOARCH}/"
        fi
        cd ..
        
        # 计算文件大小
        local size=$(du -sh "$build_dir/$output_name" | cut -f1)
        echo "✅ ${GOOS}/${GOARCH} 构建完成 (${size})"
    else
        echo "❌ ${GOOS}/${GOARCH} 构建失败"
        return 1
    fi
}

# 检查 Go 环境
if ! command -v go >/dev/null 2>&1; then
    echo "❌ Go 未安装或不在 PATH 中"
    exit 1
fi

echo "📋 Go 版本: $(go version)"
echo ""

# 下载依赖
echo "📦 下载依赖..."
go mod download
go mod tidy

# 运行测试
echo "🧪 运行测试..."
go test ./...

# 并行构建所有平台
echo "🏗️  开始并行构建..."
for platform in "${PLATFORMS[@]}"; do
    build_platform "$platform" &
done

# 等待所有构建完成
wait

echo ""
echo "🎉 构建完成！"
echo "📁 构建文件位于 dist/ 目录"
echo ""

# 显示构建结果
echo "📊 构建结果："
ls -la dist/*.{zip,tar.gz} 2>/dev/null | while read line; do
    echo "   $line"
done

echo ""
echo "🔧 使用说明:"
echo "   1. 从 dist/ 目录选择对应平台的压缩包"
echo "   2. 解压到目标系统"
echo "   3. 编辑 config.yaml 配置文件"
echo "   4. 运行程序"
echo ""
echo "✨ 构建脚本执行完成！"