package scraper

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"movie-data-capture/pkg/logger"
)

// scrapeMGStage 从MGStage抓取电影数据
func (s *Scraper) scrapeMGStage(ctx context.Context, number string) (*MovieData, error) {
	logger.Debug("Starting MGStage scraping for number: %s", number)
	
	// MGStage使用直接产品URL
	productURL := fmt.Sprintf("https://www.mgstage.com/product/product_detail/%s/", number)
	
	resp, err := s.httpClient.Get(ctx, productURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch page: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		return nil, fmt.Errorf("failed to parse HTML: %w", err)
	}
	
	movieData := &MovieData{
		Website: productURL,
		Source:  "mgstage",
		Number:  number,
	}
	
	// 提取标题
	if title := doc.Find("h1.tag").Text(); title != "" {
		movieData.Title = strings.TrimSpace(title)
	}
	
	// 提取封面图片
	if cover, exists := doc.Find("div.detail_photo img").Attr("src"); exists {
		if strings.HasPrefix(cover, "//") {
			cover = "https:" + cover
		} else if strings.HasPrefix(cover, "/") {
			cover = "https://www.mgstage.com" + cover
		}
		movieData.Cover = cover
	}
	
	// 提取演员
	var actors []string
	doc.Find("div.detail_data tbody tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("th").Text(), "出演") {
			s.Find("td a").Each(func(j int, actor *goquery.Selection) {
				if actorName := strings.TrimSpace(actor.Text()); actorName != "" {
					actors = append(actors, actorName)
				}
			})
		}
	})
	movieData.ActorList = actors
	// 将演员列表转换为逗号分隔的字符串用于Actor字段
	if len(actors) > 0 {
		movieData.Actor = strings.Join(actors, ",")
	}
	
	// 提取制作商
	doc.Find("div.detail_data tbody tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("th").Text(), "メーカー") {
			if studio := strings.TrimSpace(s.Find("td a").Text()); studio != "" {
				movieData.Studio = studio
			}
		}
	})
	
	// 提取时长
	doc.Find("div.detail_data tbody tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("th").Text(), "収録時間") {
			if runtime := strings.TrimSpace(s.Find("td").Text()); runtime != "" {
				// 仅提取数字部分
				re := regexp.MustCompile(`\d+`)
				if matches := re.FindString(runtime); matches != "" {
					movieData.Runtime = matches
				}
			}
		}
	})
	
	// 提取发布日期
	doc.Find("div.detail_data tbody tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("th").Text(), "配信開始日") {
			if release := strings.TrimSpace(s.Find("td").Text()); release != "" {
				movieData.Release = release
				movieData.Year = extractYear(release)
			}
		}
	})
	
	// 提取标签/类型
	var tags []string
	doc.Find("div.detail_data tbody tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("th").Text(), "ジャンル") {
			s.Find("td a").Each(func(j int, tag *goquery.Selection) {
				if tagName := strings.TrimSpace(tag.Text()); tagName != "" {
					tags = append(tags, tagName)
				}
			})
		}
	})
	movieData.Tag = tags
	
	// 提取导演
	doc.Find("div.detail_data tbody tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("th").Text(), "監督") {
			if director := strings.TrimSpace(s.Find("td a").Text()); director != "" {
				movieData.Director = director
			}
		}
	})
	
	// 提取系列
	doc.Find("div.detail_data tbody tr").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Find("th").Text(), "シリーズ") {
			if series := strings.TrimSpace(s.Find("td a").Text()); series != "" {
				movieData.Series = series
			}
		}
	})
	
	// 提取简介/描述
	if outline := doc.Find("div.txt").Text(); outline != "" {
		movieData.Outline = strings.TrimSpace(outline)
	}
	
	// 从样本图片提取额外剧照
	var extraFanart []string
	doc.Find("div.sample_image_wrap a").Each(func(i int, s *goquery.Selection) {
		if href, exists := s.Attr("href"); exists {
			if strings.HasPrefix(href, "//") {
				href = "https:" + href
			} else if strings.HasPrefix(href, "/") {
				href = "https://www.mgstage.com" + href
			}
			extraFanart = append(extraFanart, href)
		}
	})
	movieData.Extrafanart = extraFanart
	
	logger.Debug("Successfully scraped MGStage data for: %s", movieData.Number)
	return movieData, nil
}