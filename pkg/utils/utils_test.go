package utils

import (
	"os"
	"path/filepath"
	"testing"

	"movie-data-capture/internal/config"
)

// newScanConfig 构造一份仅用于扫描测试的配置。
// DebugMode 关闭，以便覆盖生产路径下的体积过滤行为。
func newScanConfig(sourceFolder string) *config.Config {
	cfg := &config.Config{}
	cfg.Common.SourceFolder = sourceFolder
	cfg.Common.MainMode = 1
	cfg.Common.FailedOutputFolder = filepath.Join(sourceFolder, "failed")
	cfg.Common.IgnoreFailedList = true
	cfg.Media.MediaType = ".mp4,.mkv,.strm"
	cfg.Escape.Folders = "failed, JAV_output"
	cfg.DebugMode.Switch = false
	return cfg
}

// writeFile 在指定路径写入给定字节数的文件
func writeFile(t *testing.T, path string, size int) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("failed to create dir for %s: %v", path, err)
	}
	if err := os.WriteFile(path, make([]byte, size), 0644); err != nil {
		t.Fatalf("failed to write %s: %v", path, err)
	}
}

// containsPath 判断切片中是否含有指定路径
func containsPath(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}

// TestGetMovieList_IncludesSTRMFiles 验证 .strm 文件不被体积过滤器误杀。
//
// .strm 只是一行指向真实媒体的文本路径，体积通常不足 1KB，
// 会落入「小于 120MB 即跳过」的区间而被丢弃，导致刮削时不被移动。
func TestGetMovieList_IncludesSTRMFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := newScanConfig(dir)

	strmPath := filepath.Join(dir, "MDX-0212.strm")
	writeFile(t, strmPath, 64) // 真实 .strm 的典型体积：几十字节

	list, err := GetMovieList(dir, cfg)
	if err != nil {
		t.Fatalf("GetMovieList returned error: %v", err)
	}

	if !containsPath(list, strmPath) {
		t.Errorf("small .strm file was filtered out.\n got: %v\nwant: it to contain %s", list, strmPath)
	}
}

// TestGetMovieList_StillSkipsSmallVideoFiles 确认豁免只针对 .strm，
// 普通小视频（可能是广告片段）仍应被跳过。
func TestGetMovieList_StillSkipsSmallVideoFiles(t *testing.T) {
	dir := t.TempDir()
	cfg := newScanConfig(dir)

	smallVideo := filepath.Join(dir, "advertisement.mp4")
	writeFile(t, smallVideo, 1024) // 1KB，远小于 120MB 阈值

	list, err := GetMovieList(dir, cfg)
	if err != nil {
		t.Fatalf("GetMovieList returned error: %v", err)
	}

	if containsPath(list, smallVideo) {
		t.Errorf("small .mp4 should still be skipped as a likely ad, got: %v", list)
	}
}

// TestGetMovieList_SkipsUnconfiguredExtension 确认未列入 media_type 的
// 扩展名不会因为豁免逻辑而被意外收录。
func TestGetMovieList_SkipsUnconfiguredExtension(t *testing.T) {
	dir := t.TempDir()
	cfg := newScanConfig(dir)
	cfg.Media.MediaType = ".mp4,.mkv" // 未包含 .strm

	strmPath := filepath.Join(dir, "MDX-0212.strm")
	writeFile(t, strmPath, 64)

	list, err := GetMovieList(dir, cfg)
	if err != nil {
		t.Fatalf("GetMovieList returned error: %v", err)
	}

	if containsPath(list, strmPath) {
		t.Errorf(".strm should be skipped when not in media_type, got: %v", list)
	}
}
