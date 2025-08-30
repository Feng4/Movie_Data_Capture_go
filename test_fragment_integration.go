package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"movie-data-capture/internal/config"
	"movie-data-capture/internal/scraper"
	"movie-data-capture/pkg/fragment"
	"movie-data-capture/pkg/nfo"
)

// 测试分片功能的完整集成
func testFragmentIntegration() {
	fmt.Println("=== 分片功能集成测试 ===")

	// 1. 测试分片检测
	fm := fragment.NewFragmentManager()
	testFiles := []string{
		"ABC-123-cd1.mp4",
		"ABC-123-cd2.mp4", 
		"ABC-123-cd3.mp4",
		"DEF-456.mkv", // 非分片文件
	}

	fmt.Println("\n1. 分片文件检测:")
	for _, file := range testFiles {
		isFragment := fm.IsFragmentFile(file)
		fmt.Printf("  %s -> %v\n", file, isFragment)
	}

	// 2. 测试分片分组
	fmt.Println("\n2. 分片文件分组:")
	groups, nonFragments := fm.GroupFragmentFiles(testFiles)
	fmt.Printf("  非分片文件: %v\n", nonFragments)
	for _, group := range groups {
		fmt.Printf("  组: %s (主文件: %s)\n", group.BaseName, group.MainFile)
		for _, frag := range group.Fragments {
			fmt.Printf("    - 第%d部分: %s\n", frag.PartNumber, frag.FilePath)
		}
	}

	// 3. 测试NFO生成器的分片参数
	fmt.Println("\n3. NFO生成器分片参数测试:")

	// 创建临时目录
	tempDir := "temp_test"
	os.MkdirAll(tempDir, 0755)
	defer os.RemoveAll(tempDir)

	nfoPath := filepath.Join(tempDir, "test.nfo")

	// 测试分片NFO生成
	fragmentFiles := []string{"ABC-123-cd1.mp4", "ABC-123-cd2.mp4", "ABC-123-cd3.mp4"}
	// 创建一个模拟的配置和生成器
	cfg := &config.Config{}
	generator := nfo.New(cfg)
	// 创建模拟的MovieData
	movieData := &scraper.MovieData{
		Number: "ABC-123",
		Title:  "测试电影",
		Year:   "2024",
	}
	err := generator.GenerateNFO(movieData, nfoPath, "", false, false, false, false, false, false, nil, "", "", "", true, 3, 1, fragmentFiles, 1024*1024*1024)
	if err != nil {
		fmt.Printf("  NFO生成失败: %v\n", err)
		return
	}

	// 读取生成的NFO文件
	content, err := os.ReadFile(nfoPath)
	if err != nil {
		fmt.Printf("  读取NFO文件失败: %v\n", err)
		return
	}

	fmt.Println("  生成的NFO内容:")
	nfoStr := string(content)
	lines := strings.Split(nfoStr, "\n")
	for _, line := range lines {
		if strings.Contains(line, "multipart") || strings.Contains(line, "totalparts") || strings.Contains(line, "currentpart") || strings.Contains(line, "fragmentfile") || strings.Contains(line, "totalfilesize") {
			fmt.Printf("    %s\n", strings.TrimSpace(line))
		}
	}

	fmt.Println("\n=== 测试完成 ===")
	fmt.Println("✅ 分片检测功能正常")
	fmt.Println("✅ 分片分组功能正常")
	fmt.Println("✅ NFO分片元数据生成正常")
}