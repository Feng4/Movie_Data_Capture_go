package main

import (
	"fmt"
	"movie-data-capture/pkg/fragment"
)

func testFragmentDetection() {
	fm := fragment.NewFragmentManager()
	
	// 测试文件列表
	testFiles := []string{
		"CAWD-321.CD1.mp4",
		"CAWD-321.CD2.mp4",
		"FSDSS-987[1].mp4",
		"FSDSS-987[2].mp4",
		"PRED-789-A.mp4",
		"PRED-789-B.mp4",
		"MIDE-456_1.avi",
		"MIDE-456_2.avi",
		"MIDV-654-disc1.mkv",
		"MIDV-654-disc2.mkv",
		"SSIS-001-cd1.mp4",
		"SSIS-001-cd2.mp4",
		"SSIS-001-cd3.mp4",
		"SSNI-147_part_1.mp4",
		"SSNI-147_part_2.mp4",
		"SSNI-147_part_3.mp4",
		"STARS-123-part1.mkv",
		"STARS-123-part2.mkv",
	}
	
	fmt.Println("=== 分片文件检测测试 ===")
	for _, filename := range testFiles {
		isFragment := fm.IsFragmentFile(filename)
		fmt.Printf("%-25s -> %v\n", filename, isFragment)
		
		if isFragment {
			info, err := fm.ParseFragmentInfo(filename)
			if err != nil {
				fmt.Printf("  解析错误: %v\n", err)
			} else {
				fmt.Printf("  基础名: %s, 分片号: %d, 后缀: %s\n", info.BaseName, info.PartNumber, info.PartSuffix)
			}
		}
	}
}