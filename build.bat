@echo off
REM Movie Data Capture Go 跨平台编译脚本 (Windows版本)
REM 使用方法: build.bat [版本号]

setlocal enabledelayedexpansion

REM 获取版本号
set VERSION=%1
if "%VERSION%"=="" set VERSION=dev

REM 获取构建时间
for /f "tokens=2 delims==" %%a in ('wmic OS Get localdatetime /value') do set "dt=%%a"
set "BUILD_TIME=%dt:~0,4%-%dt:~4,2%-%dt:~6,2%_%dt:~8,2%:%dt:~10,2%:%dt:~12,2%"

REM 获取Git提交
for /f %%a in ('git rev-parse --short HEAD 2^>nul') do set GIT_COMMIT=%%a
if "%GIT_COMMIT%"=="" set GIT_COMMIT=unknown

REM 项目信息
set PROJECT_NAME=mdc
set MAIN_FILE=main.go

REM 构建选项
set LDFLAGS=-s -w -X main.Version=%VERSION% -X main.BuildTime=%BUILD_TIME% -X main.GitCommit=%GIT_COMMIT%

echo.
echo 🚀 开始构建 %PROJECT_NAME% %VERSION%...
echo ⏰ 构建时间: %BUILD_TIME%
echo 🔖 Git提交: %GIT_COMMIT%
echo.

REM 清理之前的构建
if exist dist rmdir /s /q dist
mkdir dist

REM 检查 Go 环境
go version >nul 2>&1
if errorlevel 1 (
    echo ❌ Go 未安装或不在 PATH 中
    exit /b 1
)

echo 📋 Go 版本:
go version

echo.
echo 📦 下载依赖...
go mod download
go mod tidy

echo.
echo 🧪 运行测试...
go test ./...

echo.
echo 🏗️  开始构建...

REM Windows 64位
echo 🔨 构建 windows/amd64...
set GOOS=windows
set GOARCH=amd64
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-windows-amd64
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-windows-amd64\%PROJECT_NAME%-windows-amd64.exe %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-windows-amd64\
copy README.md dist\%PROJECT_NAME%-windows-amd64\
xcopy Img dist\%PROJECT_NAME%-windows-amd64\Img\ /E /I /Q >nul 2>&1

REM Windows 32位
echo 🔨 构建 windows/386...
set GOOS=windows
set GOARCH=386
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-windows-386
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-windows-386\%PROJECT_NAME%-windows-386.exe %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-windows-386\
copy README.md dist\%PROJECT_NAME%-windows-386\
xcopy Img dist\%PROJECT_NAME%-windows-386\Img\ /E /I /Q >nul 2>&1

REM Windows ARM64
echo 🔨 构建 windows/arm64...
set GOOS=windows
set GOARCH=arm64
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-windows-arm64
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-windows-arm64\%PROJECT_NAME%-windows-arm64.exe %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-windows-arm64\
copy README.md dist\%PROJECT_NAME%-windows-arm64\
xcopy Img dist\%PROJECT_NAME%-windows-arm64\Img\ /E /I /Q >nul 2>&1

REM Linux 64位
echo 🔨 构建 linux/amd64...
set GOOS=linux
set GOARCH=amd64
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-linux-amd64
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-linux-amd64\%PROJECT_NAME%-linux-amd64 %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-linux-amd64\
copy README.md dist\%PROJECT_NAME%-linux-amd64\
xcopy Img dist\%PROJECT_NAME%-linux-amd64\Img\ /E /I /Q >nul 2>&1

REM Linux 32位
echo 🔨 构建 linux/386...
set GOOS=linux
set GOARCH=386
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-linux-386
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-linux-386\%PROJECT_NAME%-linux-386 %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-linux-386\
copy README.md dist\%PROJECT_NAME%-linux-386\
xcopy Img dist\%PROJECT_NAME%-linux-386\Img\ /E /I /Q >nul 2>&1

REM Linux ARM64
echo 🔨 构建 linux/arm64...
set GOOS=linux
set GOARCH=arm64
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-linux-arm64
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-linux-arm64\%PROJECT_NAME%-linux-arm64 %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-linux-arm64\
copy README.md dist\%PROJECT_NAME%-linux-arm64\
xcopy Img dist\%PROJECT_NAME%-linux-arm64\Img\ /E /I /Q >nul 2>&1

REM macOS Intel
echo 🔨 构建 darwin/amd64...
set GOOS=darwin
set GOARCH=amd64
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-darwin-amd64
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-darwin-amd64\%PROJECT_NAME%-darwin-amd64 %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-darwin-amd64\
copy README.md dist\%PROJECT_NAME%-darwin-amd64\
xcopy Img dist\%PROJECT_NAME%-darwin-amd64\Img\ /E /I /Q >nul 2>&1

REM macOS Apple Silicon
echo 🔨 构建 darwin/arm64...
set GOOS=darwin
set GOARCH=arm64
set CGO_ENABLED=0
mkdir dist\%PROJECT_NAME%-darwin-arm64
go build -ldflags="%LDFLAGS%" -o dist\%PROJECT_NAME%-darwin-arm64\%PROJECT_NAME%-darwin-arm64 %MAIN_FILE%
copy config.yaml dist\%PROJECT_NAME%-darwin-arm64\
copy README.md dist\%PROJECT_NAME%-darwin-arm64\
xcopy Img dist\%PROJECT_NAME%-darwin-arm64\Img\ /E /I /Q >nul 2>&1

echo.
echo 📦 创建压缩包...

cd dist

REM 创建 Windows 压缩包 (需要7z或其他压缩工具)
where 7z >nul 2>&1
if not errorlevel 1 (
    echo 📦 创建 Windows 压缩包...
    7z a -tzip %PROJECT_NAME%-windows-amd64.zip %PROJECT_NAME%-windows-amd64 >nul
    7z a -tzip %PROJECT_NAME%-windows-386.zip %PROJECT_NAME%-windows-386 >nul
    7z a -tzip %PROJECT_NAME%-windows-arm64.zip %PROJECT_NAME%-windows-arm64 >nul
    echo ✅ Windows 压缩包创建完成
) else (
    echo ⚠️  7z 未找到，跳过创建 Windows 压缩包
    echo    可以手动压缩 dist\%PROJECT_NAME%-windows-* 文件夹
)

cd ..

echo.
echo 🎉 构建完成！
echo 📁 构建文件位于 dist\ 目录
echo.
echo 📊 构建结果：
dir dist\%PROJECT_NAME%-* /AD

echo.
echo 🔧 使用说明:
echo    1. 从 dist\ 目录选择对应平台的文件夹或压缩包
echo    2. 解压到目标系统 (如果是压缩包)
echo    3. 编辑 config.yaml 配置文件
echo    4. 运行程序
echo.
echo ✨ 构建脚本执行完成！

pause