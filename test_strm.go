package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"movie-data-capture/internal/config"
	"movie-data-capture/internal/scraper"
	"movie-data-capture/pkg/strm"
)

// 测试STRM文件生成功能
func testSTRMGeneration() {
	fmt.Println("=== STRM文件生成测试 ===")

	// 创建测试配置
	cfg := &config.Config{
		STRM: config.STRMConfig{
			Enable:           true,
			PathType:         "absolute",
			ContentMode:      "detailed",
			MultiPartMode:    "separate",
			NetworkBasePath:  "",
			UseWindowsPath:   false,
			ValidateFiles:    false, // 测试时不验证文件存在
			StrictValidation: false,
			OutputSuffix:     "",
		},
		NameRule: config.NameRuleConfig{
			NamingRule:  "number + '-' + title",
			MaxTitleLen: 50,
		},
		Common: config.CommonConfig{
			SourceFolder: "./test_movies",
		},
	}

	// 创建STRM生成器
	strmGen := strm.New(cfg)

	// 创建测试目录
	testDir := "test_strm_output"
	os.MkdirAll(testDir, 0755)

	fmt.Printf("1. 测试单个文件STRM生成\n")
	
	// 测试数据
	movieData := &scraper.MovieData{
		Number:  "SSIS-001",
		Title:   "美しい人妻の秘密",
		Actor:   "葵つかさ",
		Studio:  "S1 NO.1 STYLE",
		Release: "2024-01-20",
		Year:    "2024",
	}

	// 测试单个文件
	originalFile := "/home/user/movies/SSIS-001.mp4"
	err := strmGen.GenerateSTRM(movieData, originalFile, testDir)
	if err != nil {
		fmt.Printf("❌ 单文件STRM生成失败: %v\n", err)
	} else {
		fmt.Printf("✅ 单文件STRM生成成功\n")
	}

	fmt.Printf("\n2. 测试多部分文件STRM生成\n")
	
	// 测试多部分文件
	fragmentFiles := []string{
		"/home/user/movies/SSIS-001-cd1.mp4",
		"/home/user/movies/SSIS-001-cd2.mp4",
		"/home/user/movies/SSIS-001-cd3.mp4",
	}

	err = strmGen.GenerateMultiPartSTRM(movieData, fragmentFiles, testDir)
	if err != nil {
		fmt.Printf("❌ 多部分STRM生成失败: %v\n", err)
	} else {
		fmt.Printf("✅ 多部分STRM生成成功\n")
	}

	fmt.Printf("\n3. 测试不同模式\n")

	// 测试简单模式
	cfg.STRM.ContentMode = "simple"
	strmGenSimple := strm.New(cfg)
	
	movieData2 := &scraper.MovieData{
		Number: "FSDSS-987",
		Title:  "テスト映画",
		Actor:  "テスト女優",
	}
	
	err = strmGenSimple.GenerateSTRM(movieData2, "/test/movie2.mp4", testDir)
	if err != nil {
		fmt.Printf("❌ 简单模式STRM生成失败: %v\n", err)
	} else {
		fmt.Printf("✅ 简单模式STRM生成成功\n")
	}

	// 测试播放列表模式
	cfg.STRM.ContentMode = "playlist"
	cfg.STRM.MultiPartMode = "combined"
	strmGenPlaylist := strm.New(cfg)
	
	err = strmGenPlaylist.GenerateMultiPartSTRM(movieData, fragmentFiles, testDir)
	if err != nil {
		fmt.Printf("❌ 播放列表模式STRM生成失败: %v\n", err)
	} else {
		fmt.Printf("✅ 播放列表模式STRM生成成功\n")
	}

	fmt.Printf("\n4. 测试网络路径模式\n")
	
	// 测试网络路径
	cfg.STRM.PathType = "network"
	cfg.STRM.NetworkBasePath = "\\\\server\\movies"
	cfg.STRM.UseWindowsPath = true
	cfg.STRM.ContentMode = "detailed"
	strmGenNetwork := strm.New(cfg)
	
	err = strmGenNetwork.GenerateSTRM(movieData, "./local/movie.mp4", testDir)
	if err != nil {
		fmt.Printf("❌ 网络路径STRM生成失败: %v\n", err)
	} else {
		fmt.Printf("✅ 网络路径STRM生成成功\n")
	}

	fmt.Printf("\n5. 查看生成的文件\n")
	
	// 列出生成的文件
	files, err := filepath.Glob(filepath.Join(testDir, "*.strm"))
	if err != nil {
		fmt.Printf("❌ 无法读取输出目录: %v\n", err)
		return
	}

	for _, file := range files {
		fmt.Printf("📄 生成的文件: %s\n", filepath.Base(file))
		
		// 读取并显示内容（前几行）
		content, err := os.ReadFile(file)
		if err != nil {
			fmt.Printf("   ❌ 无法读取文件内容: %v\n", err)
			continue
		}
		
		lines := strings.Split(string(content), "\n")
		fmt.Printf("   内容预览:\n")
		for i, line := range lines {
			if i >= 3 && !strings.HasPrefix(line, "#") {
				fmt.Printf("   ...\n")
				break
			}
			if line != "" {
				fmt.Printf("   %s\n", line)
			}
		}
		fmt.Println()
	}
}

// 测试STRM文件验证功能
func testSTRMValidation() {
	fmt.Println("=== STRM文件验证测试 ===")
	
	// 创建测试STRM文件
	testFile := "test_validation.strm"
	content := `# Test STRM file
# Movie: Test Movie
/non/existent/path.mp4
# Comment line
http://example.com/stream.m3u8`
	
	err := os.WriteFile(testFile, []byte(content), 0644)
	if err != nil {
		fmt.Printf("❌ 创建测试文件失败: %v\n", err)
		return
	}
	defer os.Remove(testFile)
	
	// 测试验证
	cfg := &config.Config{
		STRM: config.STRMConfig{
			ValidateFiles:    true,
			StrictValidation: false,
		},
	}
	
	strmGen := strm.New(cfg)
	err = strmGen.ValidateSTRM(testFile)
	if err != nil {
		fmt.Printf("✅ 验证功能正常工作，检测到无效路径: %v\n", err)
	} else {
		fmt.Printf("⚠️  验证通过（可能是因为严格验证被禁用）\n")
	}
	
	// 测试严格验证
	cfg.STRM.StrictValidation = true
	strmGenStrict := strm.New(cfg)
	err = strmGenStrict.ValidateSTRM(testFile)
	if err != nil {
		fmt.Printf("✅ 严格验证正常工作: %v\n", err)
	} else {
		fmt.Printf("⚠️  严格验证通过\n")
	}
}

func main() {
	fmt.Println("Movie Data Capture Go - STRM功能测试工具")
	fmt.Println("==========================================")
	
	testSTRMGeneration()
	fmt.Println()
	testSTRMValidation()
	
	fmt.Println("\n=== 测试完成 ===")
	fmt.Println("请检查 test_strm_output 目录中生成的STRM文件")
	fmt.Println("\n使用建议:")
	fmt.Println("1. 根据你的媒体中心类型选择合适的配置")
	fmt.Println("2. 测试STRM文件在你的媒体中心中的播放效果")
	fmt.Println("3. 调整配置以获得最佳体验")
}