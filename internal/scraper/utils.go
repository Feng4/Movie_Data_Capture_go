package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/httpclient"
)

// extractYear 从日期字符串中提取年份
func extractYear(dateStr string) string {
	if dateStr == "" {
		return ""
	}
	
	// 尝试匹配各种日期格式并提取年份
	// 格式：YYYY-MM-DD, YYYY/MM/DD, YYYY年MM月DD日 等
	re := regexp.MustCompile(`(\d{4})`)
	matches := re.FindString(strings.TrimSpace(dateStr))
	
	if matches != "" {
		return matches
	}
	
	return ""
}

// fetchDocument 从URL获取并解析HTML文档
func fetchDocument(ctx context.Context, client *httpclient.Client, url string) (*goquery.Document, error) {
	resp, err := client.Get(ctx, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL %s: %w", url, err)
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}

	return doc, nil
}

// cleanText 清理和修剪文本内容
func cleanText(text string) string {
	return strings.TrimSpace(strings.ReplaceAll(text, "\n", " "))
}

// joinActors 用逗号分隔符连接演员姓名
func joinActors(actors []string) string {
	if len(actors) == 0 {
		return ""
	}
	return strings.Join(actors, ", ")
}