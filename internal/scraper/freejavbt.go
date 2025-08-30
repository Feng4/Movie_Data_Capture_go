package scraper

import (
	"fmt"
	"net/http"
	"regexp"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// scrapeFreeJavBT scrapes movie data from FreeJavBT website
func scrapeFreeJavBT(number string) (*MovieData, error) {
	url := fmt.Sprintf("https://freejavbt.com/%s", number)
	return scrapeFreeJavBTPage(url, number)
}

// scrapeFreeJavBTPage scrapes a specific FreeJavBT page
func scrapeFreeJavBTPage(url, originalNumber string) (*MovieData, error) {
	client := &http.Client{
		Timeout: 30 * time.Second,
	}
	
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	
	// Set headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.5")
	
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP error: %d", resp.StatusCode)
	}
	
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, err
	}
	
	// Check if page has valid content
	body := doc.Text()
	if !strings.Contains(body, "single-video-info col-12") {
		return nil, fmt.Errorf("no valid content found")
	}
	
	movieInfo := &MovieData{
		Source:  "freejavbt",
		Website: url,
	}
	
	// Extract title and number
	title, number := extractFreeJavBTTitle(doc)
	if title == "" {
		return nil, fmt.Errorf("title not found")
	}
	movieInfo.Title = title
	movieInfo.OriginalTitle = title
	movieInfo.Number = number
	
	// Extract other information
	actorList := extractFreeJavBTActor(doc)
	if len(actorList) > 0 {
		movieInfo.Actor = strings.Join(actorList, ", ")
		movieInfo.ActorList = actorList
	}
	movieInfo.ActorPhoto = extractFreeJavBTActorPhoto(actorList)
	movieInfo.Cover = extractFreeJavBTCover(doc)
	movieInfo.Release = extractFreeJavBTRelease(doc)
	movieInfo.Year = extractFreeJavBTYear(movieInfo.Release)
	movieInfo.Runtime = extractFreeJavBTRuntime(doc)
	directorList := extractFreeJavBTDirector(doc)
	if len(directorList) > 0 {
		movieInfo.Director = strings.Join(directorList, ", ")
	}
	movieInfo.Studio = extractFreeJavBTStudio(doc)
	movieInfo.Label = extractFreeJavBTPublisher(doc)
	movieInfo.Series = extractFreeJavBTSeries(doc)
	movieInfo.Tag = extractFreeJavBTTag(doc)
	movieInfo.Extrafanart = extractFreeJavBTExtraFanart(doc)
	movieInfo.Trailer = extractFreeJavBTTrailer(doc)
	
	// Set empty fields
	movieInfo.Outline = ""
	
	return movieInfo, nil
}

// extractFreeJavBTTitle extracts title and number from FreeJavBT page
func extractFreeJavBTTitle(doc *goquery.Document) (string, string) {
	titleText := strings.TrimSpace(doc.Find("title").Text())
	if titleText == "" {
		return "", ""
	}
	
	// Clean title
	titleText = strings.Replace(titleText, "| FREE JAV BT", "", -1)
	titleText = strings.TrimSpace(titleText)
	
	// Try to split by |
	parts := strings.Split(titleText, "|")
	var title, number string
	
	if len(parts) == 2 {
		number = strings.TrimSpace(parts[0])
		title = strings.TrimSpace(strings.Join(parts[1:], " "))
		title = strings.Replace(title, number, "", -1)
	} else {
		// Try to split by space
		words := strings.Fields(titleText)
		if len(words) > 2 {
			number = strings.TrimSpace(words[0])
			title = strings.TrimSpace(strings.Join(words[1:], " "))
		}
	}
	
	// Clean title
	title = strings.Replace(title, "中文字幕", "", -1)
	title = strings.Replace(title, "無碼", "", -1)
	title = strings.Replace(title, "\n", "", -1)
	title = strings.Replace(title, "_", "-", -1)
	title = strings.Replace(title, strings.ToUpper(number), "", -1)
	title = strings.Replace(title, number, "", -1)
	title = strings.Replace(title, "--", "-", -1)
	title = strings.TrimSpace(title)
	
	// Check for invalid titles
	if title == "" || strings.Contains(title, "翻译错误") || strings.Contains(title, "每日更新") {
		return "", ""
	}
	
	return title, number
}

// extractFreeJavBTActor extracts actors from FreeJavBT page
func extractFreeJavBTActor(doc *goquery.Document) []string {
	var actors []string
	
	// Male actors list to filter out
	avMen := map[string]bool{
		"貞松大輔": true, "鮫島": true, "森林原人": true, "黒田悠斗": true, "主観": true,
		"吉村卓": true, "野島誠": true, "小田切ジュン": true, "しみけん": true, "セツネヒデユキ": true,
		"大島丈": true, "玉木玲": true, "ウルフ田中": true, "ジャイアント廣田": true, "イセドン内村": true,
		"西島雄介": true, "平田司": true, "杉浦ボッ樹": true, "大沢真司": true, "ピエール剣": true,
		"羽田": true, "田淵正浩": true, "タツ": true, "南佳也": true, "吉野篤史": true,
		"今井勇太": true, "マッスル澤野": true, "井口": true, "松山伸也": true, "花岡じった": true,
		"佐川銀次": true, "およよ中野": true, "小沢とおる": true, "橋本誠吾": true, "阿部智広": true,
		"沢井亮": true, "武田大樹": true, "市川哲也": true, "???": true, "浅野あたる": true,
		"梅田吉雄": true, "阿川陽志": true, "素人": true, "結城結弦": true, "畑中哲也": true,
		"堀尾": true, "上田昌宏": true, "えりぐち": true, "市川潤": true, "沢木和也": true,
		"トニー大木": true, "横山大輔": true, "一条真斗": true, "真田京": true, "イタリアン高橋": true,
		"中田一平": true, "完全主観": true, "イェーイ高島": true, "山田万次郎": true, "澤地真人": true,
		"杉山": true, "ゴロー": true, "細田あつし": true, "藍井優太": true, "奥村友真": true,
		"ザーメン二郎": true, "桜井ちんたろう": true, "冴山トシキ": true, "久保田裕也": true, "戸川夏也": true,
		"北こうじ": true, "柏木純吉": true, "ゆうき": true, "トルティーヤ鈴木": true, "神けんたろう": true,
		"堀内ハジメ": true, "ナルシス小林": true, "アーミー": true, "池田径": true, "吉村文孝": true,
		"優生": true, "久道実": true, "一馬": true, "辻隼人": true, "片山邦生": true,
		"Qべぇ": true, "志良玉弾吾": true, "今岡爽紫郎": true, "工藤健太": true, "原口": true,
		"アベ": true, "染島貢": true, "岩下たろう": true, "小野晃": true, "たむらあゆむ": true,
		"川越将護": true, "桜木駿": true, "瀧口": true, "TJ本田": true, "園田": true,
		"宮崎": true, "鈴木一徹": true, "黒人": true, "カルロス": true, "天河": true,
		"ぷーてゃん": true, "左曲かおる": true, "富田": true, "TECH": true, "ムールかいせ": true,
		"健太": true, "山田裕二": true, "池沼ミキオ": true, "ウサミ": true, "押井敬之": true,
		"浅見草太": true, "ムータン": true, "フランクフルト林": true, "石橋豊彦": true, "矢野慎二": true,
		"芦田陽": true, "くりぼ": true, "ダイ": true, "ハッピー池田": true, "山形健": true,
		"忍野雅一": true, "渋谷優太": true, "服部義": true, "たこにゃん": true, "北山シロ": true,
		"つよぽん": true, "山本いくお": true, "学万次郎": true, "平井シンジ": true, "望月": true,
		"ゆーきゅん": true, "頭田光": true, "向理来": true, "かめじろう": true, "高橋しんと": true,
		"栗原良": true, "テツ神山": true, "タラオ": true, "真琴": true, "滝本": true,
		"金田たかお": true, "平ボンド": true, "春風ドギー": true, "桐島達也": true, "中堀健二": true,
		"徳田重男": true, "三浦屋助六": true, "志戸哲也": true, "ヒロシ": true, "オクレ": true,
		"羽目白武": true, "ジョニー岡本": true, "幸野賀一": true, "インフィニティ": true, "ジャック天野": true,
		"覆面": true, "安大吉": true, "井上亮太": true, "笹木良一": true, "艦長": true,
		"軍曹": true, "タッキー": true, "阿部ノボル": true, "ダウ兄": true, "まーくん": true,
		"梁井一": true, "カンパニー松尾": true, "大塚玉堂": true, "日比野達郎": true, "小梅": true,
		"ダイナマイト幸男": true, "タケル": true, "くるみ太郎": true, "山田伸夫": true, "氷崎健人": true,
	}
	
	doc.Find("a.actress").Each(func(i int, s *goquery.Selection) {
		actor := strings.TrimSpace(s.Text())
		if actor != "" && actor != "?" && !strings.Contains(actor, "暫無") && !avMen[actor] {
			actors = append(actors, actor)
		}
	})
	
	return actors
}

// extractFreeJavBTActorPhoto extracts actor photos (placeholder implementation)
func extractFreeJavBTActorPhoto(actors []string) map[string]string {
	actorPhoto := make(map[string]string)
	for _, actor := range actors {
		actorPhoto[actor] = ""
	}
	return actorPhoto
}

// extractFreeJavBTCover extracts cover image from FreeJavBT page
func extractFreeJavBTCover(doc *goquery.Document) string {
	selectors := []string{
		"img.video-cover.rounded.lazyload",
		"img.col-lg-2.col-md-2.col-sm-6.col-12.lazyload",
	}
	
	for _, selector := range selectors {
		img, exists := doc.Find(selector).First().Attr("data-src")
		if exists && img != "" && !strings.Contains(img, "no_preview_lg") && strings.Contains(img, "http") {
			return img
		}
	}
	return ""
}

// extractFreeJavBTRelease extracts release date from FreeJavBT page
func extractFreeJavBTRelease(doc *goquery.Document) string {
	selectors := []string{
		"span:contains('日期')",
		"span:contains('発売日')",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			if strings.Contains(s.Text(), "日期") || strings.Contains(s.Text(), "発売日") {
				// Get the next sibling element's text
				next := s.Next()
				if next.Length() > 0 {
					date := strings.TrimSpace(next.Text())
					if date != "" {
						return
					}
				}
			}
		})
	}
	return ""
}

// extractFreeJavBTYear extracts year from release date
func extractFreeJavBTYear(release string) string {
	re := regexp.MustCompile(`\d{4}`)
	matches := re.FindStringSubmatch(release)
	if len(matches) > 0 {
		return matches[0]
	}
	return ""
}

// extractFreeJavBTRuntime extracts runtime from FreeJavBT page
func extractFreeJavBTRuntime(doc *goquery.Document) string {
	selectors := []string{
		"span:contains('时长')",
		"span:contains('時長')",
		"span:contains('収録時間')",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(text, "时长") || strings.Contains(text, "時長") || strings.Contains(text, "収録時間") {
				// Get the next sibling element's text
				next := s.Next()
				if next.Length() > 0 {
					runtime := strings.TrimSpace(next.Text())
					re := regexp.MustCompile(`\d+`)
					matches := re.FindStringSubmatch(runtime)
					if len(matches) > 0 {
						return
					}
				}
			}
		})
	}
	return ""
}

// extractFreeJavBTSeries extracts series from FreeJavBT page
func extractFreeJavBTSeries(doc *goquery.Document) string {
	doc.Find("span:contains('系列')").Each(func(i int, s *goquery.Selection) {
		if strings.Contains(s.Text(), "系列") {
			// Get the next sibling element's text
			next := s.Next()
			if next.Length() > 0 {
				series := strings.TrimSpace(next.Text())
				if series != "" {
					return
				}
			}
		}
	})
	return ""
}

// extractFreeJavBTDirector extracts director from FreeJavBT page
func extractFreeJavBTDirector(doc *goquery.Document) []string {
	var directors []string
	selectors := []string{
		"span:contains('导演')",
		"span:contains('導演')",
		"span:contains('監督')",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(text, "导演") || strings.Contains(text, "導演") || strings.Contains(text, "監督") {
				// Get the next sibling element's text
				next := s.Next()
				if next.Length() > 0 {
					director := strings.TrimSpace(next.Text())
					if director != "" {
						directors = append(directors, director)
						return
					}
				}
			}
		})
		if len(directors) > 0 {
			break
		}
	}
	return directors
}

// extractFreeJavBTStudio extracts studio from FreeJavBT page
func extractFreeJavBTStudio(doc *goquery.Document) string {
	selectors := []string{
		"span:contains('制作')",
		"span:contains('製作')",
		"span:contains('メーカー')",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(text, "制作") || strings.Contains(text, "製作") || strings.Contains(text, "メーカー") {
				// Get the next sibling element's text
				next := s.Next()
				if next.Length() > 0 {
					studio := strings.TrimSpace(next.Text())
					if studio != "" {
						return
					}
				}
			}
		})
	}
	return ""
}

// extractFreeJavBTPublisher extracts publisher from FreeJavBT page
func extractFreeJavBTPublisher(doc *goquery.Document) string {
	selectors := []string{
		"span:contains('发行')",
		"span:contains('發行')",
	}
	
	for _, selector := range selectors {
		doc.Find(selector).Each(func(i int, s *goquery.Selection) {
			text := s.Text()
			if strings.Contains(text, "发行") || strings.Contains(text, "發行") {
				// Get the next sibling element's text
				next := s.Next()
				if next.Length() > 0 {
					publisher := strings.TrimSpace(next.Text())
					if publisher != "" {
						return
					}
				}
			}
		})
	}
	return ""
}

// extractFreeJavBTTag extracts tags from FreeJavBT page
func extractFreeJavBTTag(doc *goquery.Document) []string {
	var tags []string
	doc.Find("a.genre").Each(func(i int, s *goquery.Selection) {
		tag := strings.TrimSpace(s.Text())
		if tag != "" {
			tag = strings.Replace(tag, "，", "", -1)
			tags = append(tags, tag)
		}
	})
	return tags
}

// extractFreeJavBTExtraFanart extracts extra fanart from FreeJavBT page
func extractFreeJavBTExtraFanart(doc *goquery.Document) []string {
	var fanart []string
	doc.Find("a.tile-item").Each(func(i int, s *goquery.Selection) {
		href, exists := s.Attr("href")
		if exists && href != "" && !strings.Contains(href, "#preview-video") {
			fanart = append(fanart, href)
		}
	})
	return fanart
}

// extractFreeJavBTTrailer extracts trailer from FreeJavBT page
func extractFreeJavBTTrailer(doc *goquery.Document) string {
	trailer, exists := doc.Find("video#preview-video source").First().Attr("src")
	if exists && trailer != "" {
		return trailer
	}
	return ""
}

// extractFreeJavBTMosaic determines mosaic status
func extractFreeJavBTMosaic(title, actor string) string {
	combined := title + actor
	if strings.Contains(combined, "無碼") || strings.Contains(combined, "無修正") || strings.Contains(combined, "Uncensored") {
		return "无码"
	}
	return "有码"
}