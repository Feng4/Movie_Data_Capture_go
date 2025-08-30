package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/fragment"
	"movie-data-capture/pkg/parser"
)

// 测试主程序对分片文件的处理能力
func testMainProgramWithFragments() {
	fmt.Println("=== 测试主程序分片文件处理能力 ===")

	// 1. 初始化配置
	cfg := &config.Config{}
	inputPath := "test_fragments"
	outputPath := "test_output"

	// 2. 创建输出目录
	if err := os.MkdirAll(outputPath, 0755); err != nil {
		log.Printf("创建输出目录失败: %v", err)
		return
	}

	// 3. 扫描测试目录中的所有文件
	files, err := filepath.Glob(filepath.Join(inputPath, "*"))
	if err != nil {
		log.Printf("扫描文件失败: %v", err)
		return
	}

	fmt.Printf("找到 %d 个文件\n\n", len(files))

	// 4. 初始化分片管理器和解析器
	fm := fragment.NewFragmentManager()
	np := parser.NewNumberParser(cfg)

	// 5. 处理每个文件
	processedGroups := make(map[string]bool)
	for _, file := range files {
		filename := filepath.Base(file)
		fmt.Printf("处理文件: %s\n", filename)

		// 检查是否为分片文件
		isFragment := fm.IsFragmentFile(filename)
		fmt.Printf("  是否为分片文件: %v\n", isFragment)

		if isFragment {
			// 解析分片信息
			fragInfo, err := fm.ParseFragmentInfo(file)
			if err != nil {
				fmt.Printf("  解析分片信息失败: %v\n", err)
				continue
			}
			baseName := fragInfo.BaseName
			fmt.Printf("  基础名称: %s\n", baseName)

			// 检查是否已处理过这个组
			if processedGroups[baseName] {
				fmt.Printf("  跳过: 该分片组已处理\n\n")
				continue
			}

			// 标记为已处理
			processedGroups[baseName] = true

			// 获取该组的所有分片文件
			groupFiles, _ := fm.GroupFragmentFiles(files)
			for _, group := range groupFiles {
				if group.BaseName == baseName {
					fmt.Printf("  分片组包含文件: %v\n", group.GetAllFragmentPaths())
					break
				}
			}
		} else {
			// 非分片文件，直接处理
			fmt.Printf("  非分片文件，直接处理\n")
		}

		// 尝试解析番号
		number := np.GetNumber(filename)
		fmt.Printf("  解析的番号: %s\n", number)

		fmt.Println()
	}

	// 6. 显示分片分组结果
	fmt.Println("=== 分片分组结果 ===")
	groupFiles, nonFragments := fm.GroupFragmentFiles(files)
	
	fmt.Printf("分片组数量: %d\n", len(groupFiles))
	for _, group := range groupFiles {
		fmt.Printf("组 '%s': %v\n", group.BaseName, group.GetAllFragmentPaths())
	}

	fmt.Printf("\n非分片文件数量: %d\n", len(nonFragments))
	for _, file := range nonFragments {
		fmt.Printf("非分片文件: %s\n", filepath.Base(file))
	}

	// 7. 统计信息
	fmt.Println("\n=== 处理统计 ===")
	fmt.Printf("总文件数: %d\n", len(files))
	fmt.Printf("分片组数: %d\n", len(groupFiles))
	fmt.Printf("非分片文件数: %d\n", len(nonFragments))
	fmt.Printf("需要刮削的项目数: %d\n", len(groupFiles)+len(nonFragments))
}

func runTestMainProgramWithFragments() {
	testMainProgramWithFragments()
	fmt.Println("\n=== 测试完成 ===")
	fmt.Println("\n测试结果说明:")
	fmt.Println("- 分片文件应该被正确识别和分组")
	fmt.Println("- 每个分片组只应该处理一次")
	fmt.Println("- 非分片文件应该单独处理")
	fmt.Println("- 番号解析应该正常工作")
}