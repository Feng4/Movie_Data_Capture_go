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

// normalizeNumberForCompare 将番号归一化用于比对：转大写、去除非字母数字。
// 站点对番号的常见规范化（大小写、横线/下划线/空格）经此归一会消失，
// 从而只比对真正的标识部分。
func normalizeNumberForCompare(s string) string {
	return regexp.MustCompile(`[^A-Z0-9]`).ReplaceAllString(strings.ToUpper(s), "")
}

// extractDate 从混杂文本中抽出日期并统一为 YYYY-MM-DD。
// 站点的日期常与「発売日」「メーカー品番」等标签同处一个节点，
// 直接取整块文本会污染字段，故此处只取日期部分。
func extractDate(text string) string {
	if text == "" {
		return ""
	}

	// 依次匹配 YYYY-MM-DD、YYYY/MM/DD、YYYY年MM月DD日
	re := regexp.MustCompile(`(\d{4})\s*[-/年]\s*(\d{1,2})\s*[-/月]\s*(\d{1,2})`)
	m := re.FindStringSubmatch(text)
	if len(m) != 4 {
		return ""
	}

	month := m[2]
	if len(month) == 1 {
		month = "0" + month
	}
	day := m[3]
	if len(day) == 1 {
		day = "0" + day
	}

	return m[1] + "-" + month + "-" + day
}

// normalizeXCityURL 补全 XCity 返回的协议相对地址（//host/path -> https://host/path）
func normalizeXCityURL(rawURL string) string {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return ""
	}
	if strings.HasPrefix(rawURL, "//") {
		return "https:" + rawURL
	}
	if strings.HasPrefix(rawURL, "/") {
		return "https://xcity.jp" + rawURL
	}
	return rawURL
}

// isSameNumber 判断两个番号是否指向同一作品。
// 先尝试精确匹配，再按归一化形式匹配；两者都失败则判定为不同作品。
func isSameNumber(requested, got string) bool {
	if requested == "" || got == "" {
		return false
	}
	if strings.EqualFold(requested, got) {
		return true
	}
	nr := normalizeNumberForCompare(requested)
	ng := normalizeNumberForCompare(got)
	if nr == "" || ng == "" {
		return false
	}
	return nr == ng
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