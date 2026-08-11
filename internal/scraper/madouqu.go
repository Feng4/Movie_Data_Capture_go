package scraper

import (
	"context"
	"fmt"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// madouquBaseURL 是麻豆区站点的基础地址
const madouquBaseURL = "https://madouqu.com"

// madouquHeaders 返回访问麻豆区所需的请求头
func madouquHeaders() map[string]string {
	return map[string]string{
		"User-Agent":      "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Safari/537.36",
		"Accept":          "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,image/apng,*/*;q=0.8",
		"Accept-Language": "zh-CN,zh;q=0.9,en;q=0.8",
		"Connection":      "keep-alive",
		"Referer":         madouquBaseURL + "/",
	}
}

// scrapeMadouQu 从麻豆区（madouqu.com）抓取影片数据。
//
// 该站为 WordPress 架构，检索走 /?s=<关键词>，详情页为 /video/<slug>/。
// 详情页正文以「番號：」「片名：」「女郎：」等中文标签罗列字段，
// 站点混用繁简，故解析时同时匹配两种写法。
func (s *Scraper) scrapeMadouQu(ctx context.Context, number string) (*MovieData, error) {

	detailURL, matchedTitle, err := s.searchMadouQu(ctx, number)
	if err != nil {
		return nil, err
	}

	logger.Debug("MadouQu matched %q -> %s", matchedTitle, detailURL)
	return s.scrapeMadouQuPage(ctx, detailURL, number)
}

// madouquNumberVariants 生成番号的检索变体。
//
// 站点检索对分隔符敏感：MDX-0212 与 MDX0212 未必都能命中，
// 故依次尝试原始、去分隔符、以及在字母数字交界处补横线三种形式。
func madouquNumberVariants(number string) []string {
	number = strings.TrimSpace(number)
	if number == "" {
		return nil
	}

	variants := []string{number}

	// 去掉所有非字母数字字符：MDX-0212 -> MDX0212
	stripped := regexp.MustCompile(`[\W_]`).ReplaceAllString(number, "")
	if stripped != "" {
		variants = append(variants, stripped)
	}

	// 在字母与数字交界处补横线：MDX0212 -> MDX-0212
	if hyphenated := regexp.MustCompile(`^([A-Za-z]+)[-_]?(\d.*)$`).
		ReplaceAllString(stripped, "$1-$2"); hyphenated != stripped {
		variants = append(variants, hyphenated)
	}

	return dedupeStrings(variants)
}

// normalizeForMatch 归一化字符串用于番号比对：去除非字母数字并转大写
func normalizeForMatch(s string) string {
	return strings.ToUpper(regexp.MustCompile(`[\W_]`).ReplaceAllString(s, ""))
}

// searchMadouQu 依次用番号变体检索，返回首个匹配的详情页地址与标题
func (s *Scraper) searchMadouQu(ctx context.Context, number string) (string, string, error) {
	variants := madouquNumberVariants(number)
	if len(variants) == 0 {
		return "", "", fmt.Errorf("empty number for MadouQu search")
	}

	for _, variant := range variants {
		searchURL := fmt.Sprintf("%s/?s=%s", madouquBaseURL, url.QueryEscape(variant))
		logger.Debug("Trying MadouQu search: %s", searchURL)

		doc, err := s.fetchMadouQuDoc(ctx, searchURL)
		if err != nil {
			logger.Debug("MadouQu search failed for %q: %v", variant, err)
			continue
		}

		if detailURL, title, ok := matchMadouQuResult(doc, variants); ok {
			return detailURL, title, nil
		}
	}

	return "", "", fmt.Errorf("no MadouQu search result for number: %s", number)
}

// matchMadouQuResult 在检索结果中挑出与番号变体相符的条目。
//
// 结果条目有两种承载方式：封面块 div.entry-media 内的 <a>（标题在 img 的 alt 上），
// 以及标题块 h2.entry-title 内的 <a>。两者都取，以免站点改版后失效。
func matchMadouQuResult(doc *goquery.Document, variants []string) (string, string, bool) {
	type candidate struct {
		href  string
		title string
	}

	var candidates []candidate

	doc.Find("div.entry-media a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		title, _ := sel.Find("img").First().Attr("alt")
		if href != "" {
			candidates = append(candidates, candidate{href: href, title: strings.TrimSpace(title)})
		}
	})

	doc.Find("h2.entry-title a[href]").Each(func(_ int, sel *goquery.Selection) {
		href, _ := sel.Attr("href")
		if href != "" {
			candidates = append(candidates, candidate{href: href, title: strings.TrimSpace(sel.Text())})
		}
	})

	for _, c := range candidates {
		// 标题与链接都参与比对：部分条目标题不含番号，但 slug 含有
		haystacks := []string{normalizeForMatch(c.title), normalizeForMatch(c.href)}
		for _, v := range variants {
			needle := normalizeForMatch(v)
			if needle == "" {
				continue
			}
			for _, h := range haystacks {
				if strings.Contains(h, needle) {
					return c.href, c.title, true
				}
			}
		}
	}

	return "", "", false
}

// scrapeMadouQuPage 解析麻豆区详情页
func (s *Scraper) scrapeMadouQuPage(ctx context.Context, detailURL, requestedNumber string) (*MovieData, error) {
	doc, err := s.fetchMadouQuDoc(ctx, detailURL)
	if err != nil {
		return nil, err
	}

	data := parseMadouQuDetail(doc, detailURL, requestedNumber)
	if data.Title == "" && data.Number == "" {
		return nil, fmt.Errorf("failed to extract data from MadouQu page: %s", detailURL)
	}

	logger.Debug("Successfully extracted MadouQu data for: %s", data.Number)
	return data, nil
}

// fetchMadouQuDoc 获取并解析指定地址的 HTML 文档
func (s *Scraper) fetchMadouQuDoc(ctx context.Context, target string) (*goquery.Document, error) {
	resp, err := s.httpClient.Get(ctx, target, madouquHeaders())
	if err != nil {
		return nil, fmt.Errorf("failed to fetch MadouQu page: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("MadouQu returned status code %d for %s", resp.StatusCode, target)
	}

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MadouQu HTML: %w", err)
	}

	return doc, nil
}

// madouquFieldPatterns 是详情页正文中各字段的中文标签，繁简并列
var madouquFieldPatterns = struct {
	number []string
	title  []string
	actor  []string
}{
	number: []string{"番號", "番号"},
	title:  []string{"片名"},
	actor:  []string{"女郎", "女優", "女优", "演員", "演员"},
}

// parseMadouQuDetail 从详情页文档提取影片数据
func parseMadouQuDetail(doc *goquery.Document, detailURL, requestedNumber string) *MovieData {
	data := &MovieData{
		Website: detailURL,
		Source:  "madouqu",
		Tag:     []string{},
		// 站点为国产内容，无马赛克分级信息，按国产片处理
		Uncensored:  false,
		Extrafanart: []string{},
	}

	// 标题优先取正文标题，站点将番号前置于标题中
	headerTitle := strings.TrimSpace(doc.Find("div.cao_entry_header header h1").First().Text())
	if headerTitle == "" {
		headerTitle = strings.TrimSpace(doc.Find("h1.entry-title, h1").First().Text())
	}

	content := doc.Find("div.entry-content").First()

	// 正文按行拆分后逐行匹配「标签：值」
	var lines []string
	content.Find("p").Each(func(_ int, sel *goquery.Selection) {
		for _, line := range strings.Split(sel.Text(), "\n") {
			if line = strings.TrimSpace(line); line != "" {
				lines = append(lines, line)
			}
		}
	})

	var actor string
	for i, line := range lines {
		if v := madouquFieldValue(line, madouquFieldPatterns.number); v != "" && data.Number == "" {
			data.Number = v
		}
		if v := madouquFieldValue(line, madouquFieldPatterns.title); v != "" && data.Title == "" {
			data.Title = v
		}
		if actor == "" {
			// 演员值可能与标签同处一行，也可能落在下一行（站点用 <br> 分隔时）
			if v := madouquFieldValue(line, madouquFieldPatterns.actor); v != "" {
				actor = v
			} else if madouquHasLabel(line, madouquFieldPatterns.actor) && i+1 < len(lines) {
				if next := strings.TrimLeft(lines[i+1], "：: "); next != lines[i+1] {
					actor = strings.TrimSpace(next)
				}
			}
		}
	}

	// 番号兜底：正文缺失时回落到请求番号，再退到标题首段
	if data.Number == "" {
		if requestedNumber != "" {
			data.Number = requestedNumber
		} else if headerTitle != "" {
			data.Number = strings.Fields(headerTitle)[0]
		}
	}

	// 标题兜底：正文无「片名」时用页面标题去掉番号前缀
	if data.Title == "" && headerTitle != "" {
		data.Title = strings.TrimSpace(strings.Replace(headerTitle, data.Number, "", 1))
		if data.Title == "" {
			data.Title = headerTitle
		}
	}

	if actor != "" {
		actors := splitMadouQuActors(actor)
		data.ActorList = actors
		data.Actor = strings.Join(actors, ", ")
	}

	// 封面取正文首张图片，站点经 i0.wp.com CDN 代理
	content.Find("img").EachWithBreak(func(_ int, sel *goquery.Selection) bool {
		src := madouquImageSrc(sel)
		if src != "" {
			data.Cover = src
			return false
		}
		return true
	})

	// 厂牌取分类标签
	if studio := strings.TrimSpace(doc.Find("span.meta-category").First().Text()); studio != "" {
		data.Studio = studio
		data.Label = studio
	}

	// 发行日期取 <time datetime>，为 ISO 8601 带时区
	if datetime, ok := doc.Find("time[datetime]").First().Attr("datetime"); ok {
		if t, err := time.Parse(time.RFC3339, strings.TrimSpace(datetime)); err == nil {
			data.Release = t.Format("2006-01-02")
			data.Year = t.Format("2006")
		}
	}

	return data
}

// madouquHasLabel 判断该行是否含有指定标签之一
func madouquHasLabel(line string, labels []string) bool {
	for _, label := range labels {
		if strings.Contains(line, label) {
			return true
		}
	}
	return false
}

// madouquFieldValue 从「标签：值」形式的行中取出值，未命中返回空串
func madouquFieldValue(line string, labels []string) string {
	for _, label := range labels {
		idx := strings.Index(line, label)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(label):]
		// 标签与值之间可能是全角或半角冒号，也可能夹有空格
		rest = strings.TrimLeft(rest, " \t")
		if !strings.HasPrefix(rest, "：") && !strings.HasPrefix(rest, ":") {
			continue
		}
		rest = strings.TrimLeft(rest, "：: \t")
		if v := strings.TrimSpace(rest); v != "" {
			return v
		}
	}
	return ""
}

// splitMadouQuActors 按常见分隔符拆分演员名单
func splitMadouQuActors(actor string) []string {
	parts := regexp.MustCompile(`[、,，/／&\s]+`).Split(actor, -1)
	var actors []string
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			actors = append(actors, p)
		}
	}
	return dedupeStrings(actors)
}

// madouquImageSrc 取图片真实地址，跳过懒加载占位图
func madouquImageSrc(sel *goquery.Selection) string {
	for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
		src, ok := sel.Attr(attr)
		if !ok {
			continue
		}
		src = strings.TrimSpace(src)
		// 懒加载占位符为内联 base64 的 1x1 gif，需跳过
		if src == "" || strings.HasPrefix(src, "data:") {
			continue
		}
		return src
	}
	return ""
}

// dedupeStrings 去重并保持原有顺序
func dedupeStrings(items []string) []string {
	seen := make(map[string]bool, len(items))
	var result []string
	for _, item := range items {
		if item == "" || seen[item] {
			continue
		}
		seen[item] = true
		result = append(result, item)
	}
	return result
}
