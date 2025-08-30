package main

import (
	"flag"
	"fmt"
	"log"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"movie-data-capture/internal/config"
	"movie-data-capture/internal/core"
	"movie-data-capture/internal/scraper"
	"movie-data-capture/pkg/logger"
	"movie-data-capture/pkg/utils"
)

const Version = "1.0.0"

func main() {
	var (
		configPath     = flag.String("config", "config.yaml", "Config file path")
		singleFile     = flag.String("file", "", "Single movie file path")
		customNumber   = flag.String("number", "", "Custom file number")
		mainMode       = flag.Int("mode", 1, "Main mode: 1=Scraping, 2=Organizing, 3=Analysis")
		sourcePath     = flag.String("path", "", "Source folder path")
		debug          = flag.Bool("debug", false, "Enable debug mode")
		version        = flag.Bool("version", false, "Show version")
		search         = flag.String("search", "", "Search number")
		specifiedSrc   = flag.String("source", "", "Specified source")
		specifiedURL   = flag.String("url", "", "Specified URL")
		logDir         = flag.String("logdir", "", "Log directory")
	)
	flag.Parse()

	if *version {
		fmt.Printf("Movie Data Capture Go Version %s\n", Version)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Platform: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		return
	}

	// Initialize logger
	if *logDir != "" {
		logger.InitFileLogger(*logDir)
	} else {
		logger.InitConsoleLogger()
	}

	// Load configuration
	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Override config with command line flags
	if *mainMode != 1 {
		cfg.Common.MainMode = *mainMode
	}
	if *sourcePath != "" {
		cfg.Common.SourceFolder = *sourcePath
	}
	if *debug {
		cfg.DebugMode.Switch = true
	}

	printHeader()

	startTime := time.Now()
	logger.Info("Start at %s", startTime.Format("2006-01-02 15:04:05"))
	logger.Info("Load Config file '%s'", *configPath)

	if cfg.DebugMode.Switch {
		logger.Info("Debug mode enabled")
	}

	// Handle search mode
	if *search != "" {
		handleSearchMode(*search, cfg, *specifiedSrc, *specifiedURL)
		return
	}

	// Handle single file mode
	if *singleFile != "" {
		handleSingleFile(*singleFile, *customNumber, cfg, *specifiedSrc, *specifiedURL)
		return
	}

	// Handle folder processing
	handleFolderProcessing(cfg)

	endTime := time.Now()
	elapsed := endTime.Sub(startTime)
	logger.Info("Running time %v, End at %s", elapsed, endTime.Format("2006-01-02 15:04:05"))
	logger.Info("All finished!")
}

func printHeader() {
	logger.Info("================= Movie Data Capture Go =================")
	versionLine := fmt.Sprintf("Version %s", Version)
	padding := (54 - len(versionLine)) / 2
	if padding > 0 {
		versionLine = strings.Repeat(" ", padding) + versionLine
	}
	logger.Info("%s", versionLine)
	logger.Info("======================================================")
	logger.Info("Platform: %s/%s - Go %s", runtime.GOOS, runtime.GOARCH, runtime.Version())
	logger.Info("======================================================")
}

func handleSearchMode(searchTerm string, cfg *config.Config, specifiedSrc, specifiedURL string) {
	logger.Info("==================== Search Mode =====================")
	
	scraperInstance := scraper.New(cfg)
	data, err := scraperInstance.GetDataFromNumber(searchTerm, specifiedSrc, specifiedURL)
	if err != nil {
		logger.Error("Search failed: %v", err)
		return
	}
	
	if data != nil {
		logger.Info("Search result for %s:", searchTerm)
		utils.DebugPrint(data)
	} else {
		logger.Warn("No data found for %s", searchTerm)
	}
}

func handleSingleFile(filePath, customNumber string, cfg *config.Config, specifiedSrc, specifiedURL string) {
	logger.Info("==================== Single File =====================")
	
	processor := core.NewProcessor(cfg)
	
	var number string
	if customNumber != "" {
		number = customNumber
	} else {
		number = utils.GetNumberFromFilenameWithConfig(filepath.Base(filePath), cfg)
	}
	
	if number == "" {
		logger.Error("Cannot extract number from filename")
		return
	}
	
	err := processor.ProcessSingleFile(filePath, number, specifiedSrc, specifiedURL)
	if err != nil {
		logger.Error("Failed to process file %s: %v", filePath, err)
	}
}

func handleFolderProcessing(cfg *config.Config) {
	sourceFolder := cfg.Common.SourceFolder
	if sourceFolder == "" {
		sourceFolder = "."
	}
	
	processor := core.NewProcessor(cfg)
	
	movieList, err := utils.GetMovieList(sourceFolder, cfg)
	if err != nil {
		logger.Error("Failed to get movie list: %v", err)
		return
	}
	
	logger.Info("Found %d movies", len(movieList))
	logger.Info("======================================================")
	
	err = processor.ProcessMovieList(movieList)
	if err != nil {
		logger.Error("Failed to process movie list: %v", err)
	}
}