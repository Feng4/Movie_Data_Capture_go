package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config represents the application configuration
type Config struct {
	Common       CommonConfig       `yaml:"common"`
	Proxy        ProxyConfig        `yaml:"proxy"`
	NameRule     NameRuleConfig     `yaml:"name_rule"`
	Update       UpdateConfig       `yaml:"update"`
	Priority     PriorityConfig     `yaml:"priority"`
	Escape       EscapeConfig       `yaml:"escape"`
	DebugMode    DebugModeConfig    `yaml:"debug_mode"`
	Translate    TranslateConfig    `yaml:"translate"`
	Trailer      TrailerConfig      `yaml:"trailer"`
	Uncensored   UncensoredConfig   `yaml:"uncensored"`
	Media        MediaConfig        `yaml:"media"`
	Watermark    WatermarkConfig    `yaml:"watermark"`
	Extrafanart  ExtrafanartConfig  `yaml:"extrafanart"`
	Storyline    StorylineConfig    `yaml:"storyline"`
	CCConvert    CCConvertConfig    `yaml:"cc_convert"`
	Javdb        JavdbConfig        `yaml:"javdb"`
	Face         FaceConfig         `yaml:"face"`
	Jellyfin     JellyfinConfig     `yaml:"jellyfin"`
	ActorPhoto   ActorPhotoConfig   `yaml:"actor_photo"`
}

type CommonConfig struct {
	MainMode                   int    `yaml:"main_mode"`
	SourceFolder               string `yaml:"source_folder"`
	FailedOutputFolder         string `yaml:"failed_output_folder"`
	SuccessOutputFolder        string `yaml:"success_output_folder"`
	LinkMode                   int    `yaml:"link_mode"`
	ScanHardlink               bool   `yaml:"scan_hardlink"`
	FailedMove                 bool   `yaml:"failed_move"`
	AutoExit                   bool   `yaml:"auto_exit"`
	TranslateToSC              bool   `yaml:"translate_to_sc"`
	ActorGender                string `yaml:"actor_gender"`
	DelEmptyFolder             bool   `yaml:"del_empty_folder"`
	NFOSkipDays                int    `yaml:"nfo_skip_days"`
	IgnoreFailedList           bool   `yaml:"ignore_failed_list"`
	DownloadOnlyMissingImages  bool   `yaml:"download_only_missing_images"`
	MappingTableValidity       int    `yaml:"mapping_table_validity"`
	Jellyfin                   int    `yaml:"jellyfin"`
	ActorOnlyTag               bool   `yaml:"actor_only_tag"`
	Sleep                      int    `yaml:"sleep"`
	AnonymousFill              int    `yaml:"anonymous_fill"`
	MultiThreading             int    `yaml:"multi_threading"`
	StopCounter                int    `yaml:"stop_counter"`
	RerunDelay                 string `yaml:"rerun_delay"`
}

type ProxyConfig struct {
	Switch     bool   `yaml:"switch"`
	Proxy      string `yaml:"proxy"`
	Timeout    int    `yaml:"timeout"`
	Retry      int    `yaml:"retry"`
	Type       string `yaml:"type"`
	CACertFile string `yaml:"cacert_file"`
}

type NameRuleConfig struct {
	LocationRule           string `yaml:"location_rule"`
	NamingRule             string `yaml:"naming_rule"`
	MaxTitleLen            int    `yaml:"max_title_len"`
	ImageNamingWithNumber  bool   `yaml:"image_naming_with_number"`
	NumberUppercase        bool   `yaml:"number_uppercase"`
	NumberRegexs           string `yaml:"number_regexs"`
}

type UpdateConfig struct {
	UpdateCheck bool `yaml:"update_check"`
}

type PriorityConfig struct {
	Website string `yaml:"website"`
}

type EscapeConfig struct {
	Literals string `yaml:"literals"`
	Folders  string `yaml:"folders"`
}

type DebugModeConfig struct {
	Switch bool `yaml:"switch"`
}

type TranslateConfig struct {
	Switch        bool   `yaml:"switch"`
	Engine        string `yaml:"engine"`
	TargetLang    string `yaml:"target_language"`
	Key           string `yaml:"key"`
	Delay         int    `yaml:"delay"`
	Values        string `yaml:"values"`
	ServiceSite   string `yaml:"service_site"`
}

type TrailerConfig struct {
	Switch bool `yaml:"switch"`
}

type UncensoredConfig struct {
	UncensoredPrefix string `yaml:"uncensored_prefix"`
}

type MediaConfig struct {
	MediaType string `yaml:"media_type"`
	SubType   string `yaml:"sub_type"`
}

type WatermarkConfig struct {
	Switch bool `yaml:"switch"`
	Water  int  `yaml:"water"`
}

type ExtrafanartConfig struct {
	Switch           bool   `yaml:"switch"`
	ExtrafanartFolder string `yaml:"extrafanart_folder"`
	ParallelDownload int    `yaml:"parallel_download"`
}

type StorylineConfig struct {
	Switch         bool   `yaml:"switch"`
	Site           string `yaml:"site"`
	CensoredSite   string `yaml:"censored_site"`
	UncensoredSite string `yaml:"uncensored_site"`
	ShowResult     int    `yaml:"show_result"`
	RunMode        int    `yaml:"run_mode"`
}

type CCConvertConfig struct {
	Mode int    `yaml:"mode"`
	Vars string `yaml:"vars"`
}

type JavdbConfig struct {
	Sites string `yaml:"sites"`
}

type FaceConfig struct {
	LocationsModel  string  `yaml:"locations_model"`
	UncensoredOnly  bool    `yaml:"uncensored_only"`
	AlwaysImagecut  bool    `yaml:"always_imagecut"`
	AspectRatio     float64 `yaml:"aspect_ratio"`
}

type JellyfinConfig struct {
	MultiPartFanart bool `yaml:"multi_part_fanart"`
}

type ActorPhotoConfig struct {
	DownloadForKodi bool `yaml:"download_for_kodi"`
}

// Load loads configuration from file
func Load(configPath string) (*Config, error) {
	// Search for config file in multiple locations
	searchPaths := []string{
		configPath,
		filepath.Join(".", "config.yaml"),
		filepath.Join(".", "config.yml"),
		filepath.Join(os.Getenv("HOME"), "mdc.yaml"),
		filepath.Join(os.Getenv("HOME"), ".mdc.yaml"),
		filepath.Join(os.Getenv("HOME"), ".mdc", "config.yaml"),
		filepath.Join(os.Getenv("HOME"), ".config", "mdc", "config.yaml"),
	}

	var actualPath string
	for _, path := range searchPaths {
		if _, err := os.Stat(path); err == nil {
			actualPath = path
			break
		}
	}

	if actualPath == "" {
		// Create default config if not found
		return createDefaultConfig(searchPaths[1])
	}

	data, err := os.ReadFile(actualPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	config := &Config{}
	err = yaml.Unmarshal(data, config)
	if err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return config, nil
}

// createDefaultConfig creates a default configuration file
func createDefaultConfig(path string) (*Config, error) {
	config := &Config{
		Common: CommonConfig{
			MainMode:                  1,
			SourceFolder:              "./",
			FailedOutputFolder:        "failed",
			SuccessOutputFolder:       "JAV_output",
			LinkMode:                  0,
			ScanHardlink:              false,
			FailedMove:                true,
			AutoExit:                  false,
			TranslateToSC:             true,
			ActorGender:               "female",
			DelEmptyFolder:            true,
			NFOSkipDays:               30,
			IgnoreFailedList:          false,
			DownloadOnlyMissingImages: true,
			MappingTableValidity:      7,
			Jellyfin:                  0,
			ActorOnlyTag:              false,
			Sleep:                     3,
			AnonymousFill:             0,
			MultiThreading:            0,
			StopCounter:               0,
			RerunDelay:                "0",
		},
		Proxy: ProxyConfig{
			Switch:  false,
			Proxy:   "",
			Timeout: 5,
			Retry:   3,
			Type:    "socks5",
		},
		NameRule: NameRuleConfig{
			LocationRule:          "actor + '/' + number",
			NamingRule:            "number + '-' + title",
			MaxTitleLen:           50,
			ImageNamingWithNumber: false,
			NumberUppercase:       false,
		},
		Update: UpdateConfig{
			UpdateCheck: true,
		},
		Priority: PriorityConfig{
			Website: "javbus,javdb,fanza,xcity,mgstage,fc2,fc2club,avsox,jav321",
		},
		Escape: EscapeConfig{
			Literals: "\\()/",
			Folders:  "failed, JAV_output",
		},
		DebugMode: DebugModeConfig{
			Switch: false,
		},
		Translate: TranslateConfig{
			Switch:      false,
			Engine:      "google-free",
			TargetLang:  "zh_cn",
			Delay:       1,
			Values:      "title,outline",
			ServiceSite: "translate.google.cn",
		},
		Trailer: TrailerConfig{
			Switch: false,
		},
		Uncensored: UncensoredConfig{
			UncensoredPrefix: "S2M,BT,LAF,SMD",
		},
		Media: MediaConfig{
			MediaType: ".mp4,.avi,.rmvb,.wmv,.mov,.mkv,.flv,.ts,.webm,.iso",
			SubType:   ".smi,.srt,.idx,.sub,.sup,.psb,.ssa,.ass,.usf,.xss,.ssf,.rt,.lrc,.sbv,.vtt,.ttml",
		},
		Watermark: WatermarkConfig{
			Switch: true,
			Water:  2,
		},
		Extrafanart: ExtrafanartConfig{
			Switch:            true,
			ExtrafanartFolder: "extrafanart",
			ParallelDownload:  1,
		},
		Storyline: StorylineConfig{
			Switch:         true,
			Site:           "1:avno1",
			CensoredSite:   "5:xcity,6:amazon",
			UncensoredSite: "3:58avgo",
			ShowResult:     0,
			RunMode:        1,
		},
		CCConvert: CCConvertConfig{
			Mode: 1,
			Vars: "actor,director,label,outline,series,studio,tag,title",
		},
		Javdb: JavdbConfig{
			Sites: "38,39",
		},
		Face: FaceConfig{
			LocationsModel: "hog",
			UncensoredOnly: true,
			AlwaysImagecut: false,
			AspectRatio:    2.12,
		},
		Jellyfin: JellyfinConfig{
			MultiPartFanart: false,
		},
		ActorPhoto: ActorPhotoConfig{
			DownloadForKodi: false,
		},
	}

	// Write default config to file
	data, err := yaml.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal default config: %w", err)
	}

	err = os.WriteFile(path, data, 0644)
	if err != nil {
		return nil, fmt.Errorf("failed to write default config: %w", err)
	}

	return config, nil
}

// GetSources returns list of sources from priority config
func (c *Config) GetSources() []string {
	return strings.Split(c.Priority.Website, ",")
}

// GetMediaTypes returns list of supported media file extensions
func (c *Config) GetMediaTypes() []string {
	types := strings.Split(strings.ToLower(c.Media.MediaType), ",")
	for i, t := range types {
		types[i] = strings.TrimSpace(t)
	}
	return types
}

// GetSubTypes returns list of supported subtitle file extensions  
func (c *Config) GetSubTypes() []string {
	types := strings.Split(strings.ToLower(c.Media.SubType), ",")
	for i, t := range types {
		types[i] = strings.TrimSpace(t)
	}
	return types
}

// ParseRerunDelay parses rerun delay string to seconds
func (c *Config) ParseRerunDelay() int {
	value := c.Common.RerunDelay
	if value == "" || value == "0" {
		return 0
	}

	// If it's just a number, treat as seconds
	if seconds, err := strconv.Atoi(value); err == nil {
		return seconds
	}

	// Parse format like "1h30m45s"
	total := 0
	value = strings.ToLower(value)
	
	// Extract hours
	if idx := strings.Index(value, "h"); idx != -1 {
		if h, err := strconv.Atoi(value[:idx]); err == nil {
			total += h * 3600
		}
		value = value[idx+1:]
	}
	
	// Extract minutes
	if idx := strings.Index(value, "m"); idx != -1 {
		if m, err := strconv.Atoi(value[:idx]); err == nil {
			total += m * 60
		}
		value = value[idx+1:]
	}
	
	// Extract seconds
	if idx := strings.Index(value, "s"); idx != -1 {
		if s, err := strconv.Atoi(value[:idx]); err == nil {
			total += s
		}
	}

	return total
}