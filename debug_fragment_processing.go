package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"

	"movie-data-capture/pkg/fragment"
)

// 调试分片处理逻辑
func debugFragmentProcessing() {
	fmt.Println("=== 调试分片处理逻辑 ===")

	// 1. 扫描测试目录中的所有文件
	files, err := filepath.Glob(filepath.Join("test_fragments", "*"))
	if err != nil {
		log.Printf("扫描文件失败: %v", err)
		return
	}

	fmt.Printf("找到 %d 个文件\n", len(files))
	for _, file := range files {
		fmt.Printf("  %s\n", file)
	}

	// 2. 初始化分片管理器
	fm := fragment.NewFragmentManager()

	// 3. 分组分片文件
	fragmentGroups, nonFragmentFiles := fm.GroupFragmentFiles(files)

	fmt.Printf("\n分片组数量: %d\n", len(fragmentGroups))
	fmt.Printf("非分片文件数量: %d\n", len(nonFragmentFiles))

	// 4. 详细显示每个分片组的信息
	for i, group := range fragmentGroups {
		fmt.Printf("\n=== 分片组 %d ===\n", i+1)
		fmt.Printf("基础名称: %s\n", group.BaseName)
		fmt.Printf("主文件: %s\n", group.GetMainFileFromGroup())
		fmt.Printf("分片数量: %d\n", group.GetFragmentCount())
		fmt.Printf("分片文件:\n")
		
		for j, fragInfo := range group.Fragments {
			fmt.Printf("  分片 %d:\n", j+1)
			fmt.Printf("    文件路径: %s\n", fragInfo.FilePath)
			fmt.Printf("    基础名称: %s\n", fragInfo.BaseName)
			fmt.Printf("    分片编号: %d\n", fragInfo.PartNumber)
			fmt.Printf("    分片后缀: %s\n", fragInfo.PartSuffix)
			fmt.Printf("    扩展名: %s\n", fragInfo.Extension)
			
			// 检查文件是否存在
			if _, err := os.Stat(fragInfo.FilePath); os.IsNotExist(err) {
				fmt.Printf("    状态: 文件不存在！\n")
			} else {
				fmt.Printf("    状态: 文件存在\n")
			}
		}
	}

	// 5. 模拟主程序的处理逻辑
	fmt.Printf("\n=== 模拟主程序处理逻辑 ===\n")
	for i, group := range fragmentGroups {
		fmt.Printf("\n处理分片组 %d: %s\n", i+1, group.BaseName)
		fmt.Printf("主文件: %s\n", group.GetMainFileFromGroup())
		fmt.Printf("将移动 %d 个分片文件:\n", len(group.Fragments))
		
		for j, fragInfo := range group.Fragments {
			fmt.Printf("  分片 %d: %s\n", j+1, fragInfo.FilePath)
			
			// 检查源文件是否存在
			if _, err := os.Stat(fragInfo.FilePath); os.IsNotExist(err) {
				fmt.Printf("    -> 跳过: 文件已移动或缺失\n")
			} else {
				fmt.Printf("    -> 准备移动到目标目录\n")
			}
		}
	}

	fmt.Println("\n=== 调试完成 ===")
}