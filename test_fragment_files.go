package main

import (
	"fmt"
	"os"
	"path/filepath"
)

// 创建测试用的分片影片文件
func createTestFragmentFiles() {
	// 创建测试目录
	testDir := "test_fragments"
	if err := os.MkdirAll(testDir, 0755); err != nil {
		fmt.Printf("创建测试目录失败: %v\n", err)
		return
	}

	// 测试用的分片影片文件名
	testFiles := []string{
		// SSIS系列分片
		"SSIS-001-cd1.mp4",
		"SSIS-001-cd2.mp4",
		"SSIS-001-cd3.mp4",
		
		// STARS系列分片
		"STARS-123-part1.mkv",
		"STARS-123-part2.mkv",
		
		// MIDE系列分片
		"MIDE-456_1.avi",
		"MIDE-456_2.avi",
		
		// PRED系列分片
		"PRED-789-A.mp4",
		"PRED-789-B.mp4",
		
		// CAWD系列分片
		"CAWD-321.CD1.mp4",
		"CAWD-321.CD2.mp4",
		
		// MIDV系列分片
		"MIDV-654-disc1.mkv",
		"MIDV-654-disc2.mkv",
		
		// FSDSS系列分片
		"FSDSS-987[1].mp4",
		"FSDSS-987[2].mp4",
		
		// SSNI系列分片
		"SSNI-147_part_1.mp4",
		"SSNI-147_part_2.mp4",
		"SSNI-147_part_3.mp4",
		
		// 非分片文件（用于对比测试）
		"SSIS-002.mp4",
		"STARS-124.mkv",
		"MIDE-457.avi",
	}

	// 创建测试文件
	for _, filename := range testFiles {
		filePath := filepath.Join(testDir, filename)
		file, err := os.Create(filePath)
		if err != nil {
			fmt.Printf("创建文件 %s 失败: %v\n", filename, err)
			continue
		}
		
		// 写入一些测试内容
		content := fmt.Sprintf("这是测试文件: %s\n创建时间: %s\n", filename, "2024-01-20")
		file.WriteString(content)
		file.Close()
		
		fmt.Printf("创建测试文件: %s\n", filename)
	}

	fmt.Printf("\n总共创建了 %d 个测试文件\n", len(testFiles))
	fmt.Printf("测试文件位于目录: %s\n", testDir)
}

func runCreateTestFragmentFiles() {
	fmt.Println("=== 创建分片影片测试文件 ===")
	createTestFragmentFiles()
	fmt.Println("\n=== 测试文件创建完成 ===")
	fmt.Println("\n现在可以使用这些文件测试主程序的分片影片刮削功能")
	fmt.Println("建议的测试步骤:")
	fmt.Println("1. 运行主程序并指向 test_fragments 目录")
	fmt.Println("2. 观察程序是否正确识别分片文件")
	fmt.Println("3. 检查是否只对每组分片影片刮削一次")
	fmt.Println("4. 验证生成的NFO文件是否包含分片信息")
}