package nfo

import (
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"movie-data-capture/internal/config"
	"movie-data-capture/internal/scraper"
	"movie-data-capture/pkg/logger"
)

// Movie 表示NFO XML结构
type Movie struct {
	XMLName         xml.Name `xml:"movie"`
	Title           string   `xml:"title"`
	OriginalTitle   string   `xml:"originaltitle"`
	SortTitle       string   `xml:"sorttitle"`
	CustomRating    string   `xml:"customrating"`
	MPAA            string   `xml:"mpaa"`
	Set             string   `xml:"set"`
	Studio          string   `xml:"studio"`
	Year            string   `xml:"year"`
	Outline         string   `xml:"outline"`
	Plot            string   `xml:"plot"`
	Runtime         string   `xml:"runtime"`
	Director        string   `xml:"director"`
	Poster          string   `xml:"poster"`
	Thumb           string   `xml:"thumb"`
	Fanart          string   `xml:"fanart,omitempty"`
	Actors          []Actor  `xml:"actor,omitempty"`
	Maker           string   `xml:"maker"`
	Label           string   `xml:"label"`
	Tags            []string `xml:"tag,omitempty"`
	Genres          []string `xml:"genre,omitempty"`
	Number          string   `xml:"num"`
	Premiered       string   `xml:"premiered"`
	ReleaseDate     string   `xml:"releasedate"`
	Release         string   `xml:"release"`
	UserRating      string   `xml:"userrating,omitempty"`
	Rating          string   `xml:"rating,omitempty"`
	CriticRating    string   `xml:"criticrating,omitempty"`
	Ratings         *Ratings `xml:"ratings,omitempty"`
	Cover           string   `xml:"cover"`
	Trailer         string   `xml:"trailer,omitempty"`
	Website         string   `xml:"website"`
	// 分片相关字段
	IsMultiPart     bool     `xml:"ismultipart,omitempty"`
	TotalParts      int      `xml:"totalparts,omitempty"`
	CurrentPart     int      `xml:"currentpart,omitempty"`
	FragmentFiles   []string `xml:"fragmentfile,omitempty"`
	TotalFileSize   int64    `xml:"totalfilesize,omitempty"`
}

// Actor 表示NFO中的演员
type Actor struct {
	Name  string `xml:"name"`
	Thumb string `xml:"thumb,omitempty"`
}

// Ratings 表示评分信息
type Ratings struct {
	Rating RatingInfo `xml:"rating"`
}

// RatingInfo 表示详细的评分信息
type RatingInfo struct {
	Name    string  `xml:"name,attr"`
	Max     string  `xml:"max,attr"`
	Default string  `xml:"default,attr"`
	Value   float64 `xml:"value"`
	Votes   int     `xml:"votes"`
}

// Generator 处理NFO文件生成
type Generator struct {
	config *config.Config
}

// New 创建一个新的NFO生成器
func New(cfg *config.Config) *Generator {
	return &Generator{
		config: cfg,
	}
}

// GenerateNFO 为电影数据生成NFO文件
func (g *Generator) GenerateNFO(data *scraper.MovieData, outputPath, part string, chineseSubtitle, leak, uncensored, hack, fourK, iso bool, actorList []string, posterPath, thumbPath, fanartPath string, isMultiPart bool, totalParts, currentPart int, fragmentFiles []string, totalFileSize int64) error {
	// 确定NFO文件路径
	var nfoPath string
	if g.config.Common.MainMode == 3 {
		// 模式3：NFO必须与视频文件名完全匹配
		nfoPath = strings.TrimSuffix(outputPath, filepath.Ext(outputPath)) + ".nfo"
	} else {
		// 其他模式：使用基于编号的命名
		leakWord := ""
		if leak {
			leakWord = "-leak"
		}
		
		cWord := ""
		if chineseSubtitle {
			cWord = "-C"
		}
		
		hackWord := ""
		if hack {
			hackWord = "-hack"
		}
		
		// 如果是hack或leak，不使用C后缀
		if len(hackWord) > 0 || len(leakWord) > 0 {
			cWord = ""
		}
		
		nfoPath = filepath.Join(outputPath, fmt.Sprintf("%s%s%s%s%s.nfo", data.Number, part, leakWord, cWord, hackWord))
	}

	// 读取现有NFO以保留用户评分（如果存在）
	var existingRating string
	var existingCriticRating string
	var existingRatings *Ratings
	
	if _, err := os.Stat(nfoPath); err == nil {
		if existing, err := g.readExistingNFO(nfoPath); err == nil {
			existingRating = existing.Rating
			existingCriticRating = existing.CriticRating
			existingRatings = existing.Ratings
		}
	}

	// 创建电影结构
	movie := &Movie{
		Title:         data.NamingRule,
		OriginalTitle: data.OriginalNaming,
		SortTitle:     data.NamingRule,
		CustomRating:  "JP-18+",
		MPAA:          "JP-18+",
		Set:           data.Series,
		Studio:        data.Studio,
		Year:          data.Year,
		Runtime:       strings.ReplaceAll(data.Runtime, " ", ""),
		Director:      data.Director,
		Poster:        posterPath,
		Thumb:         thumbPath,
		Maker:         data.Studio,
		Label:         data.Label,
		Number:        data.Number,
		Premiered:     data.Release,
		ReleaseDate:   data.Release,
		Release:       data.Release,
		Cover:         data.Cover,
		Website:       data.Website,
		// 分片相关字段
		IsMultiPart:   isMultiPart,
		TotalParts:    totalParts,
		CurrentPart:   currentPart,
		FragmentFiles: fragmentFiles,
		TotalFileSize: totalFileSize,
	}

	// 设置概要和剧情
	outline := data.Outline
	if outline == "" {
		// 保持为空
	} else if data.Source == "pissplay" {
		// 对于pissplay，直接使用概要
		movie.Outline = outline
		movie.Plot = outline
	} else {
		// 对于其他来源，添加编号前缀
		movie.Outline = fmt.Sprintf("%s#%s", data.Number, outline)
		movie.Plot = movie.Outline
	}

	// 为非Jellyfin设置fanart
	if g.config.Common.Jellyfin == 0 {
		movie.Fanart = fanartPath
	}

	// 添加演员
	for _, actorName := range actorList {
		actor := Actor{Name: actorName}
		if data.ActorPhoto != nil {
			if thumb, exists := data.ActorPhoto[actorName]; exists {
				actor.Thumb = thumb
			}
		}
		movie.Actors = append(movie.Actors, actor)
	}

	// 添加标签和类型
	if g.config.Common.Jellyfin == 0 {
		if g.config.Common.ActorOnlyTag {
			// 仅添加演员名称作为标签
			for _, actor := range actorList {
				movie.Tags = append(movie.Tags, actor)
			}
		} else {
			// 添加各种标签
			if chineseSubtitle {
				movie.Tags = append(movie.Tags, "中文字幕")
			}
			if leak {
				movie.Tags = append(movie.Tags, "流出")
			}
			if uncensored {
				movie.Tags = append(movie.Tags, "无码")
			}
			if hack {
				movie.Tags = append(movie.Tags, "破解")
			}
			if fourK {
				movie.Tags = append(movie.Tags, "4k")
			}
			if iso {
				movie.Tags = append(movie.Tags, "原盘")
			}
			
			// 添加自定义标签
			movie.Tags = append(movie.Tags, data.Tag...)
		}
	}

	// 添加类型
	if chineseSubtitle {
		movie.Genres = append(movie.Genres, "中文字幕")
	}
	if leak {
		movie.Genres = append(movie.Genres, "无码流出")
	}
	if uncensored {
		movie.Genres = append(movie.Genres, "无码")
	}
	if hack {
		movie.Genres = append(movie.Genres, "破解")
	}
	if fourK {
		movie.Genres = append(movie.Genres, "4k")
	}
	if iso {
		movie.Genres = append(movie.Genres, "原盘")
	}
	
	// 添加自定义类型
	movie.Genres = append(movie.Genres, data.Tag...)

	// 处理评分
	if existingRating != "" {
		movie.UserRating = existingRating
	}
	
	if data.UserRating > 0 {
		movie.Rating = fmt.Sprintf("%.1f", data.UserRating*2.0)
		movie.CriticRating = fmt.Sprintf("%.1f", data.UserRating*20.0)
		movie.Ratings = &Ratings{
			Rating: RatingInfo{
				Name:    "javdb",
				Max:     "5",
				Default: "true",
				Value:   data.UserRating,
				Votes:   data.UserVotes,
			},
		}
	} else if existingCriticRating != "" || existingRatings != nil {
		movie.Rating = existingRating
		movie.CriticRating = existingCriticRating
		movie.Ratings = existingRatings
	}

	// Add trailer if enabled
	if g.config.Trailer.Switch && data.Trailer != "" {
		movie.Trailer = data.Trailer
	}

	// Write NFO file
	return g.writeNFO(nfoPath, movie)
}

// writeNFO 以适当的格式写入NFO文件
func (g *Generator) writeNFO(filePath string, movie *Movie) error {
	// Create directory if needed
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory: %w", err)
	}

	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create NFO file: %w", err)
	}
	defer file.Close()

	// Write XML header
	file.WriteString(`<?xml version="1.0" encoding="UTF-8" ?>` + "\n")

	// For Jellyfin, use simple text nodes; for others, use CDATA
	if g.config.Common.Jellyfin > 0 {
		// Jellyfin mode: simple XML
		encoder := xml.NewEncoder(file)
		encoder.Indent("", "  ")
		err = encoder.Encode(movie)
	} else {
		// KODI mode: with CDATA sections
		err = g.writeKodiNFO(file, movie)
	}

	if err != nil {
		return fmt.Errorf("failed to write NFO content: %w", err)
	}

	logger.Info("Generated NFO: %s", filepath.Base(filePath))
	return nil
}

// writeKodiNFO 为KODI写入带有CDATA部分的NFO
func (g *Generator) writeKodiNFO(file *os.File, movie *Movie) error {
	write := func(format string, args ...interface{}) {
		file.WriteString(fmt.Sprintf(format, args...))
	}

	write("<movie>\n")
	write("  <title><![CDATA[%s]]></title>\n", movie.Title)
	write("  <originaltitle><![CDATA[%s]]></originaltitle>\n", movie.OriginalTitle)
	write("  <sorttitle><![CDATA[%s]]></sorttitle>\n", movie.SortTitle)
	write("  <customrating>%s</customrating>\n", movie.CustomRating)
	write("  <mpaa>%s</mpaa>\n", movie.MPAA)
	write("  <set>%s</set>\n", movie.Set)
	write("  <studio>%s</studio>\n", movie.Studio)
	write("  <year>%s</year>\n", movie.Year)
	write("  <outline><![CDATA[%s]]></outline>\n", movie.Outline)
	write("  <plot><![CDATA[%s]]></plot>\n", movie.Plot)
	write("  <runtime>%s</runtime>\n", movie.Runtime)
	write("  <director>%s</director>\n", movie.Director)
	write("  <poster>%s</poster>\n", movie.Poster)
	write("  <thumb>%s</thumb>\n", movie.Thumb)
	
	if movie.Fanart != "" {
		write("  <fanart>%s</fanart>\n", movie.Fanart)
	}

	// Write actors
	for _, actor := range movie.Actors {
		write("  <actor>\n")
		write("    <name>%s</name>\n", actor.Name)
		if actor.Thumb != "" {
			write("    <thumb>%s</thumb>\n", actor.Thumb)
		}
		write("  </actor>\n")
	}

	write("  <maker>%s</maker>\n", movie.Maker)
	write("  <label>%s</label>\n", movie.Label)

	// Write tags
	for _, tag := range movie.Tags {
		write("  <tag>%s</tag>\n", tag)
	}

	// Write genres
	for _, genre := range movie.Genres {
		write("  <genre>%s</genre>\n", genre)
	}

	write("  <num>%s</num>\n", movie.Number)
	write("  <premiered>%s</premiered>\n", movie.Premiered)
	write("  <releasedate>%s</releasedate>\n", movie.ReleaseDate)
	write("  <release>%s</release>\n", movie.Release)

	// Write ratings
	if movie.UserRating != "" {
		write("  <userrating>%s</userrating>\n", movie.UserRating)
	}
	if movie.Rating != "" {
		write("  <rating>%s</rating>\n", movie.Rating)
	}
	if movie.CriticRating != "" {
		write("  <criticrating>%s</criticrating>\n", movie.CriticRating)
	}
	if movie.Ratings != nil {
		write("  <ratings>\n")
		write("    <rating name=\"%s\" max=\"%s\" default=\"%s\">\n", 
			movie.Ratings.Rating.Name, movie.Ratings.Rating.Max, movie.Ratings.Rating.Default)
		write("      <value>%.1f</value>\n", movie.Ratings.Rating.Value)
		write("      <votes>%d</votes>\n", movie.Ratings.Rating.Votes)
		write("    </rating>\n")
		write("  </ratings>\n")
	}

	write("  <cover>%s</cover>\n", movie.Cover)
	
	if movie.Trailer != "" {
		write("  <trailer>%s</trailer>\n", movie.Trailer)
	}
	
	write("  <website>%s</website>\n", movie.Website)

	// Write fragment information if applicable
	if movie.IsMultiPart {
		write("  <ismultipart>true</ismultipart>\n")
		write("  <totalparts>%d</totalparts>\n", movie.TotalParts)
		write("  <currentpart>%d</currentpart>\n", movie.CurrentPart)
		write("  <totalfilesize>%d</totalfilesize>\n", movie.TotalFileSize)
		for _, fragmentFile := range movie.FragmentFiles {
			write("  <fragmentfile>%s</fragmentfile>\n", fragmentFile)
		}
	}

	write("</movie>\n")

	return nil
}

// readExistingNFO 读取现有的NFO文件以保留用户数据
func (g *Generator) readExistingNFO(filePath string) (*Movie, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, err
	}

	// Parse XML to extract rating information
	content := string(data)
	movie := &Movie{}

	// Extract user rating
	if match := regexp.MustCompile(`<userrating>([^<]+)</userrating>`).FindStringSubmatch(content); len(match) > 1 {
		if regexp.MustCompile(`^\d+(\.\d+)?$`).MatchString(strings.TrimSpace(match[1])) {
			movie.UserRating = strings.TrimSpace(match[1])
		}
	}

	// Extract rating
	if match := regexp.MustCompile(`<rating>([^<]+)</rating>`).FindStringSubmatch(content); len(match) > 1 {
		if regexp.MustCompile(`^\d+(\.\d+)?$`).MatchString(strings.TrimSpace(match[1])) {
			movie.Rating = strings.TrimSpace(match[1])
		}
	}

	// Extract critic rating
	if match := regexp.MustCompile(`<criticrating>([^<]+)</criticrating>`).FindStringSubmatch(content); len(match) > 1 {
		if regexp.MustCompile(`^\d+(\.\d+)?$`).MatchString(strings.TrimSpace(match[1])) {
			movie.CriticRating = strings.TrimSpace(match[1])
		}
	}

	// Extract ratings block
	if match := regexp.MustCompile(`<ratings>.*?<rating name="javdb"[^>]*>.*?<value>([^<]+)</value>.*?<votes>([^<]+)</votes>.*?</rating>.*?</ratings>`).FindStringSubmatch(content); len(match) > 2 {
		if value, err := strconv.ParseFloat(match[1], 64); err == nil {
			if votes, err := strconv.Atoi(match[2]); err == nil {
				movie.Ratings = &Ratings{
					Rating: RatingInfo{
						Name:    "javdb",
						Max:     "5",
						Default: "true",
						Value:   value,
						Votes:   votes,
					},
				}
			}
		}
	}

	return movie, nil
}