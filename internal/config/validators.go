package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

// BasicConfigValidator provides basic configuration validation
type BasicConfigValidator struct{}

// NewBasicConfigValidator creates a new basic config validator
func NewBasicConfigValidator() *BasicConfigValidator {
	return &BasicConfigValidator{}
}

// Validate validates the configuration
func (v *BasicConfigValidator) Validate(config *Config) error {
	if err := v.validateCommon(&config.Common); err != nil {
		return fmt.Errorf("common config validation failed: %w", err)
	}

	if err := v.validateProxy(&config.Proxy); err != nil {
		return fmt.Errorf("proxy config validation failed: %w", err)
	}

	if err := v.validateNameRule(&config.NameRule); err != nil {
		return fmt.Errorf("name rule config validation failed: %w", err)
	}

	if err := v.validateTranslate(&config.Translate); err != nil {
		return fmt.Errorf("translate config validation failed: %w", err)
	}

	if err := v.validateFace(&config.Face); err != nil {
		return fmt.Errorf("face config validation failed: %w", err)
	}

	if err := v.validateMedia(&config.Media); err != nil {
		return fmt.Errorf("media config validation failed: %w", err)
	}

	return nil
}

// validateCommon validates common configuration
func (v *BasicConfigValidator) validateCommon(config *CommonConfig) error {
	// Validate main mode
	if config.MainMode < 0 || config.MainMode > 3 {
		return fmt.Errorf("invalid main_mode: %d, must be 0-3", config.MainMode)
	}

	// Validate link mode
	if config.LinkMode < 0 || config.LinkMode > 2 {
		return fmt.Errorf("invalid link_mode: %d, must be 0-2", config.LinkMode)
	}

	// Validate source folder exists
	if config.SourceFolder != "" {
		if _, err := os.Stat(config.SourceFolder); os.IsNotExist(err) {
			return fmt.Errorf("source folder does not exist: %s", config.SourceFolder)
		}
	}

	// Validate numeric ranges
	if config.Sleep < 0 {
		return fmt.Errorf("sleep must be non-negative, got: %d", config.Sleep)
	}

	if config.MultiThreading < 0 {
		return fmt.Errorf("multi_threading must be non-negative, got: %d", config.MultiThreading)
	}

	if config.NFOSkipDays < 0 {
		return fmt.Errorf("nfo_skip_days must be non-negative, got: %d", config.NFOSkipDays)
	}

	if config.MappingTableValidity < 1 {
		return fmt.Errorf("mapping_table_validity must be positive, got: %d", config.MappingTableValidity)
	}

	// Validate actor gender
	validGenders := []string{"female", "male", "both", ""}
	if !v.contains(validGenders, config.ActorGender) {
		return fmt.Errorf("invalid actor_gender: %s, must be one of: %v", config.ActorGender, validGenders)
	}

	// Validate rerun delay format
	if config.RerunDelay != "" && config.RerunDelay != "0" {
		if err := v.validateTimeFormat(config.RerunDelay); err != nil {
			return fmt.Errorf("invalid rerun_delay format: %w", err)
		}
	}

	return nil
}

// validateProxy validates proxy configuration
func (v *BasicConfigValidator) validateProxy(config *ProxyConfig) error {
	if !config.Switch {
		return nil // Skip validation if proxy is disabled
	}

	// Validate proxy URL format
	if config.Proxy != "" {
		if _, err := url.Parse(config.Proxy); err != nil {
			return fmt.Errorf("invalid proxy URL: %s, error: %w", config.Proxy, err)
		}
	}

	// Validate timeout
	if config.Timeout <= 0 {
		return fmt.Errorf("proxy timeout must be positive, got: %d", config.Timeout)
	}

	// Validate retry count
	if config.Retry < 0 {
		return fmt.Errorf("proxy retry must be non-negative, got: %d", config.Retry)
	}

	// Validate proxy type
	validTypes := []string{"http", "https", "socks5", "socks4"}
	if !v.contains(validTypes, config.Type) {
		return fmt.Errorf("invalid proxy type: %s, must be one of: %v", config.Type, validTypes)
	}

	// Validate CA cert file if specified
	if config.CACertFile != "" {
		if _, err := os.Stat(config.CACertFile); os.IsNotExist(err) {
			return fmt.Errorf("CA cert file does not exist: %s", config.CACertFile)
		}
	}

	return nil
}

// validateNameRule validates name rule configuration
func (v *BasicConfigValidator) validateNameRule(config *NameRuleConfig) error {
	// Validate max title length
	if config.MaxTitleLen <= 0 {
		return fmt.Errorf("max_title_len must be positive, got: %d", config.MaxTitleLen)
	}

	// Validate location rule contains required variables
	if config.LocationRule != "" {
		if !strings.Contains(config.LocationRule, "number") {
			return fmt.Errorf("location_rule must contain 'number' variable")
		}
	}

	// Validate naming rule contains required variables
	if config.NamingRule != "" {
		if !strings.Contains(config.NamingRule, "number") {
			return fmt.Errorf("naming_rule must contain 'number' variable")
		}
	}

	// Validate number regex if specified
	if config.NumberRegexs != "" {
		regexes := strings.Split(config.NumberRegexs, ",")
		for _, regex := range regexes {
			regex = strings.TrimSpace(regex)
			if regex != "" {
				if _, err := regexp.Compile(regex); err != nil {
					return fmt.Errorf("invalid number regex '%s': %w", regex, err)
				}
			}
		}
	}

	return nil
}

// validateTranslate validates translate configuration
func (v *BasicConfigValidator) validateTranslate(config *TranslateConfig) error {
	if !config.Switch {
		return nil // Skip validation if translate is disabled
	}

	// Validate engine
	validEngines := []string{"google-free", "google", "baidu", "youdao", "deepl"}
	if !v.contains(validEngines, config.Engine) {
		return fmt.Errorf("invalid translate engine: %s, must be one of: %v", config.Engine, validEngines)
	}

	// Validate target language
	validLangs := []string{"zh_cn", "zh_tw", "en", "ja", "ko", "fr", "de", "es", "ru"}
	if !v.contains(validLangs, config.TargetLang) {
		return fmt.Errorf("invalid target language: %s, must be one of: %v", config.TargetLang, validLangs)
	}

	// Validate delay
	if config.Delay < 0 {
		return fmt.Errorf("translate delay must be non-negative, got: %d", config.Delay)
	}

	// Validate values
	if config.Values != "" {
		validValues := []string{"title", "outline", "tag", "series", "studio", "director", "actor"}
		values := strings.Split(config.Values, ",")
		for _, value := range values {
			value = strings.TrimSpace(value)
			if value != "" && !v.contains(validValues, value) {
				return fmt.Errorf("invalid translate value: %s, must be one of: %v", value, validValues)
			}
		}
	}

	// Validate service site URL if specified
	if config.ServiceSite != "" {
		if _, err := url.Parse("https://" + config.ServiceSite); err != nil {
			return fmt.Errorf("invalid service site: %s, error: %w", config.ServiceSite, err)
		}
	}

	return nil
}

// validateFace validates face configuration
func (v *BasicConfigValidator) validateFace(config *FaceConfig) error {
	// Validate locations model
	validModels := []string{"hog", "cnn", ""}
	if !v.contains(validModels, config.LocationsModel) {
		return fmt.Errorf("invalid locations_model: %s, must be one of: %v", config.LocationsModel, validModels)
	}

	// Validate aspect ratio
	if config.AspectRatio <= 0 {
		return fmt.Errorf("aspect_ratio must be positive, got: %f", config.AspectRatio)
	}

	if config.AspectRatio > 10 {
		return fmt.Errorf("aspect_ratio seems too large: %f, maximum recommended is 10", config.AspectRatio)
	}

	return nil
}

// validateMedia validates media configuration
func (v *BasicConfigValidator) validateMedia(config *MediaConfig) error {
	// Validate media types format
	if config.MediaType != "" {
		types := strings.Split(config.MediaType, ",")
		for _, mediaType := range types {
			mediaType = strings.TrimSpace(mediaType)
			if mediaType != "" && !strings.HasPrefix(mediaType, ".") {
				return fmt.Errorf("media type must start with dot: %s", mediaType)
			}
		}
	}

	// Validate subtitle types format
	if config.SubType != "" {
		types := strings.Split(config.SubType, ",")
		for _, subType := range types {
			subType = strings.TrimSpace(subType)
			if subType != "" && !strings.HasPrefix(subType, ".") {
				return fmt.Errorf("subtitle type must start with dot: %s", subType)
			}
		}
	}

	return nil
}

// validateTimeFormat validates time format like "1h30m45s"
func (v *BasicConfigValidator) validateTimeFormat(timeStr string) error {
	// If it's just a number, it's valid (seconds)
	if _, err := strconv.Atoi(timeStr); err == nil {
		return nil
	}

	// Validate format like "1h30m45s"
	pattern := `^(\d+h)?(\d+m)?(\d+s)?$`
	matched, err := regexp.MatchString(pattern, strings.ToLower(timeStr))
	if err != nil {
		return err
	}

	if !matched {
		return fmt.Errorf("invalid time format: %s, expected format like '1h30m45s' or number of seconds", timeStr)
	}

	return nil
}

// contains checks if a slice contains a string
func (v *BasicConfigValidator) contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// PathConfigValidator validates file and directory paths
type PathConfigValidator struct {
	CreateMissingDirs bool
}

// NewPathConfigValidator creates a new path config validator
func NewPathConfigValidator(createMissingDirs bool) *PathConfigValidator {
	return &PathConfigValidator{
		CreateMissingDirs: createMissingDirs,
	}
}

// Validate validates path-related configuration
func (v *PathConfigValidator) Validate(config *Config) error {
	// Validate and optionally create output directories
	paths := []string{
		config.Common.SuccessOutputFolder,
		config.Common.FailedOutputFolder,
	}

	if config.Extrafanart.Switch && config.Extrafanart.ExtrafanartFolder != "" {
		paths = append(paths, config.Extrafanart.ExtrafanartFolder)
	}

	for _, path := range paths {
		if path == "" {
			continue
		}

		// Convert relative paths to absolute
		if !filepath.IsAbs(path) {
			wd, err := os.Getwd()
			if err != nil {
				return fmt.Errorf("failed to get working directory: %w", err)
			}
			path = filepath.Join(wd, path)
		}

		// Check if directory exists
		if _, err := os.Stat(path); os.IsNotExist(err) {
			if v.CreateMissingDirs {
				if err := os.MkdirAll(path, 0755); err != nil {
					return fmt.Errorf("failed to create directory %s: %w", path, err)
				}
			} else {
				return fmt.Errorf("directory does not exist: %s", path)
			}
		} else if err != nil {
			return fmt.Errorf("failed to check directory %s: %w", path, err)
		}
	}

	return nil
}

// NetworkConfigValidator validates network-related configuration
type NetworkConfigValidator struct{}

// NewNetworkConfigValidator creates a new network config validator
func NewNetworkConfigValidator() *NetworkConfigValidator {
	return &NetworkConfigValidator{}
}

// Validate validates network-related configuration
func (v *NetworkConfigValidator) Validate(config *Config) error {
	// Validate proxy configuration if enabled
	if config.Proxy.Switch {
		if config.Proxy.Proxy == "" {
			return fmt.Errorf("proxy is enabled but proxy URL is empty")
		}

		// Test proxy connectivity (optional, can be expensive)
		// This could be made configurable
	}

	// Validate timeout values
	if config.Proxy.Timeout > 300 {
		return fmt.Errorf("proxy timeout too large: %d seconds, maximum recommended is 300", config.Proxy.Timeout)
	}

	// Validate retry count
	if config.Proxy.Retry > 10 {
		return fmt.Errorf("proxy retry count too large: %d, maximum recommended is 10", config.Proxy.Retry)
	}

	return nil
}

// PerformanceConfigValidator validates performance-related configuration
type PerformanceConfigValidator struct{}

// NewPerformanceConfigValidator creates a new performance config validator
func NewPerformanceConfigValidator() *PerformanceConfigValidator {
	return &PerformanceConfigValidator{}
}

// Validate validates performance-related configuration
func (v *PerformanceConfigValidator) Validate(config *Config) error {
	// Validate multi-threading settings
	if config.Common.MultiThreading > 20 {
		return fmt.Errorf("multi_threading too high: %d, maximum recommended is 20", config.Common.MultiThreading)
	}

	// Validate sleep settings
	if config.Common.Sleep > 60 {
		return fmt.Errorf("sleep too high: %d seconds, maximum recommended is 60", config.Common.Sleep)
	}

	// Validate extrafanart parallel download
	if config.Extrafanart.Switch && config.Extrafanart.ParallelDownload > 10 {
		return fmt.Errorf("extrafanart parallel_download too high: %d, maximum recommended is 10", config.Extrafanart.ParallelDownload)
	}

	// Validate translate delay
	if config.Translate.Switch && config.Translate.Delay > 30 {
		return fmt.Errorf("translate delay too high: %d seconds, maximum recommended is 30", config.Translate.Delay)
	}

	return nil
}