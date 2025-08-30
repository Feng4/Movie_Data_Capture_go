package core

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"movie-data-capture/internal/config"
	"movie-data-capture/internal/scraper"
	"movie-data-capture/pkg/downloader"
	"movie-data-capture/pkg/fragment"
	"movie-data-capture/pkg/imageprocessor"
	"movie-data-capture/pkg/logger"
	"movie-data-capture/pkg/nfo"
	"movie-data-capture/pkg/storage"
	"movie-data-capture/pkg/utils"
	"movie-data-capture/pkg/watermark"
)

// Processor handles the core movie processing logic
type Processor struct {
	config        *config.Config
	scraper       *scraper.Scraper
	downloader    *downloader.Downloader
	storage       *storage.Storage
	nfoGen        *nfo.Generator
	watermark     *watermark.WatermarkProcessor
	imageProcessor *imageprocessor.ImageProcessor
	fragmentMgr   *fragment.FragmentManager

	// Concurrency control
	semaphore  chan struct{}
	wg         sync.WaitGroup
	processMux sync.Mutex
	processed  int
	failed     int
}

// ProcessResult represents the result of processing a movie
type ProcessResult struct {
	FilePath string
	Number   string
	Success  bool
	Error    error
}

// ProcessItem represents an item to be processed (either a single file or a fragment group)
type ProcessItem struct {
	FilePath      string
	IsFragment    bool
	FragmentGroup *fragment.FragmentGroup
}

// NewProcessor creates a new processor instance
func NewProcessor(cfg *config.Config) *Processor {
	// Create semaphore for concurrency control
	maxWorkers := cfg.Common.MultiThreading
	if maxWorkers <= 0 {
		maxWorkers = 1 // Sequential processing
	}

	p := &Processor{
		config:        cfg,
		scraper:       scraper.New(cfg),
		downloader:    downloader.New(cfg),
		storage:       storage.New(cfg),
		nfoGen:        nfo.New(cfg),
		watermark:     watermark.NewWatermarkProcessor(cfg),
		imageProcessor: imageprocessor.NewImageProcessor(cfg),
		fragmentMgr:   fragment.NewFragmentManager(),
		semaphore:     make(chan struct{}, maxWorkers),
	}

	return p
}

// ProcessSingleFile processes a single movie file
func (p *Processor) ProcessSingleFile(filePath, number, specifiedSource, specifiedURL string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	logger.Info("Processing single file: %s", filePath)
	logger.Info("Using number: %s", number)

	result := p.processMovie(ctx, filePath, number, specifiedSource, specifiedURL)
	if result.Error != nil {
		return fmt.Errorf("failed to process %s: %w", filePath, result.Error)
	}

	logger.Info("Successfully processed: %s", filePath)
	return nil
}

// processMovieWithFragment processes a movie with fragment context
func (p *Processor) processMovieWithFragment(ctx context.Context, item ProcessItem, number, customNumber, customUrl string) ProcessResult {
	result := ProcessResult{
		FilePath: item.FilePath,
		Number:   number,
	}

	// Parse movie flags from the main file
	flags := utils.ParseMovieFlags(filepath.Base(item.FilePath))
	
	// Prepare fragment information
	var isMultiPart bool
	var totalParts, currentPart int
	var fragmentFiles []string
	var totalFileSize int64
	
	// If this is a fragment group, collect all fragment information
	if item.IsFragment && item.FragmentGroup != nil {
		// Process using the main file but with fragment context
		logger.Debug("Processing fragment group '%s' with %d parts", 
			item.FragmentGroup.BaseName, item.FragmentGroup.GetFragmentCount())
		
		// Set fragment information
		isMultiPart = true
		totalParts = item.FragmentGroup.GetFragmentCount()
		currentPart = 1 // Main file is typically the first part
		
		// Collect all fragment file paths
		for _, fragFile := range item.FragmentGroup.Fragments {
				fragmentFiles = append(fragmentFiles, filepath.Base(fragFile.FilePath))
				if info, err := os.Stat(fragFile.FilePath); err == nil {
				totalFileSize += info.Size()
			}
		}
		
		// Update flags
		flags.Part = fmt.Sprintf("1-%d", totalParts)
		flags.IsMultiPart = true
		
		logger.Info("Fragment group '%s': %d parts, total size: %.2f MB", 
			item.FragmentGroup.BaseName, totalParts, float64(totalFileSize)/(1024*1024))
	}

	// Check if uncensored
	uncensored := utils.IsUncensored(number, p.config)

	// Get movie data from scraper
	movieData, err := p.scraper.GetDataFromNumber(number, customNumber, customUrl)
	if err != nil {
		result.Error = fmt.Errorf("failed to scrape data: %w", err)
		p.handleFailedFile(item.FilePath)
		return result
	}

	if movieData == nil {
		result.Error = fmt.Errorf("no movie data found")
		p.handleFailedFile(item.FilePath)
		return result
	}

	// Debug print if enabled
	if p.config.DebugMode.Switch {
		utils.DebugPrint(movieData)
	}

	// Determine processing mode and call appropriate method with fragment info
	switch p.config.Common.MainMode {
	case 1:
		// Scraping mode
		err = p.processScrapingModeWithFragment(ctx, item.FilePath, movieData, flags, uncensored, isMultiPart, totalParts, currentPart, fragmentFiles, totalFileSize, item.FragmentGroup)
	case 2:
		// Organizing mode
		err = p.processOrganizingModeWithFragment(item.FilePath, movieData, flags, isMultiPart, totalParts, currentPart, fragmentFiles, totalFileSize, item.FragmentGroup)
	case 3:
		// Analysis mode
		err = p.processAnalysisModeWithFragment(ctx, item.FilePath, movieData, flags, uncensored, isMultiPart, totalParts, currentPart, fragmentFiles, totalFileSize, item.FragmentGroup)
	default:
		err = fmt.Errorf("unsupported main mode: %d", p.config.Common.MainMode)
	}

	if err != nil {
		result.Error = err
		p.handleFailedFile(item.FilePath)
		return result
	}

	result.Success = true
	return result
}

// ProcessMovieList processes a list of movie files with concurrency control and fragment handling
func (p *Processor) ProcessMovieList(movieList []string) error {
	if len(movieList) == 0 {
		logger.Info("No movies to process")
		return nil
	}

	// Apply stop counter if configured
	stopCounter := p.config.Common.StopCounter
	if stopCounter > 0 && stopCounter < len(movieList) {
		movieList = movieList[:stopCounter]
		logger.Info("Processing limited to %d movies due to stop counter", stopCounter)
	}

	// 分片文件预处理：分组分片文件，避免重复刮削
	fragmentGroups, nonFragmentFiles := p.fragmentMgr.GroupFragmentFiles(movieList)
	
	logger.Info("Found %d fragment groups and %d individual files", len(fragmentGroups), len(nonFragmentFiles))
	
	// 创建处理队列：分片组的主文件 + 非分片文件
	processQueue := make([]ProcessItem, 0, len(fragmentGroups)+len(nonFragmentFiles))
	
	// 添加分片组的主文件到处理队列
	for i, group := range fragmentGroups {
		if group.HasMissingParts() {
			logger.Warn("Fragment group '%s' has missing parts, processing anyway", group.BaseName)
		}
		
		// 创建group的副本以避免指针问题
		groupCopy := fragmentGroups[i]
		processQueue = append(processQueue, ProcessItem{
			FilePath:      group.GetMainFileFromGroup(),
			IsFragment:    true,
			FragmentGroup: &groupCopy,
		})
		
		logger.Info("Will process fragment group '%s' using main file: %s (%d parts)", 
			group.BaseName, filepath.Base(group.GetMainFileFromGroup()), group.GetFragmentCount())
	}
	
	// 添加非分片文件到处理队列
	for _, filePath := range nonFragmentFiles {
		processQueue = append(processQueue, ProcessItem{
			FilePath:   filePath,
			IsFragment: false,
		})
	}

	// Create context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Channel for results
	resultChan := make(chan ProcessResult, len(processQueue))

	// Process movies with concurrency control
	for i, item := range processQueue {
		// Acquire semaphore slot
		p.semaphore <- struct{}{}

		// Extract number from filename
		number := utils.GetNumberFromFilename(filepath.Base(item.FilePath))
		if number == "" {
			logger.Warn("Could not extract number from: %s", item.FilePath)
			<-p.semaphore // Release semaphore
			continue
		}

		// Add to wait group and start processing
		p.wg.Add(1)
		go func(processItem ProcessItem, num string, index int) {
			defer func() {
				<-p.semaphore // Release semaphore
				p.wg.Done()
			}()

			// Show progress
			percentage := float64(index+1) / float64(len(processQueue)) * 100
			if processItem.IsFragment {
				logger.Info("Processing [%.1f%% %d/%d] Fragment Group: %s (%d parts)", 
					percentage, index+1, len(processQueue), 
					filepath.Base(processItem.FilePath), processItem.FragmentGroup.GetFragmentCount())
			} else {
				logger.Info("Processing [%.1f%% %d/%d] %s", 
					percentage, index+1, len(processQueue), filepath.Base(processItem.FilePath))
			}

			// Add processing delay
			if p.config.Common.Sleep > 0 {
				time.Sleep(time.Duration(p.config.Common.Sleep) * time.Second)
			}

			// Process the movie (with fragment context)
			result := p.processMovieWithFragment(ctx, processItem, num, "", "")
			resultChan <- result
		}(item, number, i)
	}

	// Close result channel when all goroutines complete
	go func() {
		p.wg.Wait()
		close(resultChan)
	}()

	// Collect results
	for result := range resultChan {
		p.processMux.Lock()
		if result.Success {
			p.processed++
		} else {
			p.failed++
			logger.Error("Failed to process %s: %v", result.FilePath, result.Error)
		}
		p.processMux.Unlock()
	}

	logger.Info("Processing completed: %d successful, %d failed", p.processed, p.failed)

	// Clean up empty folders if configured
	if p.config.Common.DelEmptyFolder {
		p.cleanupEmptyFolders()
	}

	return nil
}

// processMovie processes a single movie file (internal method)
func (p *Processor) processMovie(ctx context.Context, filePath, number, specifiedSource, specifiedURL string) ProcessResult {
	result := ProcessResult{
		FilePath: filePath,
		Number:   number,
	}

	// Parse movie flags from filename
	flags := utils.ParseMovieFlags(filePath)

	// Check if uncensored
	uncensored := utils.IsUncensored(number, p.config)

	// Get movie data from scraper
	movieData, err := p.scraper.GetDataFromNumber(number, specifiedSource, specifiedURL)
	if err != nil {
		result.Error = fmt.Errorf("failed to scrape data: %w", err)
		p.handleFailedFile(filePath)
		return result
	}

	if movieData == nil {
		result.Error = fmt.Errorf("no movie data found")
		p.handleFailedFile(filePath)
		return result
	}

	// Debug print if enabled
	if p.config.DebugMode.Switch {
		utils.DebugPrint(movieData)
	}

	// Determine processing mode
	switch p.config.Common.MainMode {
	case 1:
		// Scraping mode
		err = p.processScrapingMode(ctx, filePath, movieData, flags.Part, flags.Leak, flags.ChineseSubtitle, flags.Hack, flags.FourK, flags.ISO, uncensored)
	case 2:
		// Organizing mode
		err = p.processOrganizingMode(filePath, movieData, flags.Part, flags.Leak, flags.ChineseSubtitle, flags.Hack, flags.FourK, flags.ISO)
	case 3:
		// Analysis mode (scraping in place)
		err = p.processAnalysisMode(ctx, filePath, movieData, flags.Part, flags.Leak, flags.ChineseSubtitle, flags.Hack, flags.FourK, flags.ISO, uncensored)
	default:
		err = fmt.Errorf("unsupported main mode: %d", p.config.Common.MainMode)
	}

	if err != nil {
		result.Error = err
		p.handleFailedFile(filePath)
		return result
	}

	result.Success = true
	return result
}

// processScrapingModeWithFragment handles mode 1 (scraping with moving files) with fragment support
func (p *Processor) processScrapingModeWithFragment(ctx context.Context, filePath string, data *scraper.MovieData, flags utils.MovieFlags, uncensored bool, isMultiPart bool, totalParts, currentPart int, fragmentFiles []string, totalFileSize int64, fragmentGroup *fragment.FragmentGroup) error {
	// Create output folder
	outputPath, err := p.storage.CreateFolder(data)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	// Download images and generate file names
	ext := utils.GetImageExtension(data.Cover)
	var fanartPath, posterPath, thumbPath string
	
	if p.config.NameRule.ImageNamingWithNumber {
		// Use number-based naming
		leakWord := ""
		if flags.Leak {
			leakWord = "-leak"
		}
		cWord := ""
		if flags.ChineseSubtitle && !flags.Hack && !flags.Leak {
			cWord = "-C"
		}
		hackWord := ""
		if flags.Hack {
			hackWord = "-hack"
		}
		
		prefix := data.Number + leakWord + cWord + hackWord
		fanartPath = prefix + "-fanart" + ext
		posterPath = prefix + "-poster" + ext
		thumbPath = prefix + "-thumb" + ext
	} else {
		// Use simple naming
		fanartPath = "fanart" + ext
		posterPath = "poster" + ext
		thumbPath = "thumb" + ext
	}

	// Download cover image
	fullThumbPath := filepath.Join(outputPath, thumbPath)
	if data.Cover != "" {
		err = p.downloader.DownloadCover(ctx, data.Cover, fullThumbPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download cover: %v", err)
		} else {
			// Create fanart copy for non-Jellyfin
			if p.config.Common.Jellyfin == 0 {
				fullFanartPath := filepath.Join(outputPath, fanartPath)
				// Copy thumb to fanart (simplified, in real implementation you'd copy the file)
				p.downloader.DownloadCover(ctx, data.Cover, fullFanartPath, data.Headers)
			}
		}
	}

	// Download small cover if needed
	if data.ImageCut == 3 && data.CoverSmall != "" {
		smallCoverPath := filepath.Join(outputPath, posterPath)
		err = p.downloader.DownloadCover(ctx, data.CoverSmall, smallCoverPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download small cover: %v", err)
		}
	}

	// Download extra fanart (only for main part or single file)
	if (flags.Part == "" || strings.ToLower(flags.Part) == "-cd1") && p.config.Extrafanart.Switch && len(data.Extrafanart) > 0 {
		err = p.downloader.DownloadExtrafanart(ctx, data.Extrafanart, outputPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download extrafanart: %v", err)
		}
	}

	// Download trailer if enabled
	if (flags.Part == "" || strings.ToLower(flags.Part) == "-cd1") && p.config.Trailer.Switch && data.Trailer != "" {
		trailerName := fmt.Sprintf("%s%s-trailer.mp4", data.Number, getFileSuffix(flags.Leak, flags.ChineseSubtitle, flags.Hack))
		trailerPath := filepath.Join(outputPath, trailerName)
		err = p.downloader.DownloadTrailer(ctx, data.Trailer, trailerPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download trailer: %v", err)
		}
	}

	// Download actor photos if enabled
	if (flags.Part == "" || strings.ToLower(flags.Part) == "-cd1") && p.config.ActorPhoto.DownloadForKodi && len(data.ActorPhoto) > 0 {
		err = p.downloader.DownloadActorPhotos(ctx, data.ActorPhoto, outputPath)
		if err != nil {
			logger.Warn("Failed to download actor photos: %v", err)
		}
	}

	// Perform image cutting/cropping
	logger.Debug("Image cutting check: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	
	// Check if this is FC2 content - FC2 numbers don't need image cutting
	isFC2 := strings.HasPrefix(strings.ToUpper(data.Number), "FC2")
	if isFC2 {
		logger.Debug("Skipping image cutting for FC2 content: %s", data.Number)
		// For FC2, copy the same image to poster path (fanart, thumb, poster are the same)
		if fullThumbPath != "" && posterPath != "" {
			fullPosterPath := filepath.Join(outputPath, posterPath)
			err = p.imageProcessor.CopyImage(fullThumbPath, fullPosterPath)
			if err != nil {
				logger.Warn("Failed to copy image for FC2: %v", err)
			} else {
				logger.Info("Successfully copied image for FC2: %s -> %s", fullThumbPath, fullPosterPath)
			}
		}
	} else if data.ImageCut != 0 || p.config.Face.AlwaysImagecut {
		// Determine if we should skip face recognition
		skipFaceRec := p.config.Face.UncensoredOnly && !uncensored
		
		// Use imagecut from data, or 1 if always_imagecut is enabled
		imagecut := data.ImageCut
		if p.config.Face.AlwaysImagecut {
			imagecut = 1
		}
		
		logger.Debug("Performing image cutting: imagecut=%d, skipFaceRec=%v", imagecut, skipFaceRec)
		logger.Debug("Paths: fanart=%s, poster=%s", fullThumbPath, filepath.Join(outputPath, posterPath))
		
		// Only cut if we have both fanart and poster paths
		if fullThumbPath != "" && posterPath != "" {
			err = p.imageProcessor.CutImage(imagecut, fullThumbPath, filepath.Join(outputPath, posterPath), skipFaceRec)
			if err != nil {
				logger.Warn("Failed to cut image: %v", err)
			} else {
				logger.Info("Successfully cut image: %s -> %s", fullThumbPath, filepath.Join(outputPath, posterPath))
			}
		}
	} else {
		logger.Debug("Skipping image cutting: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	}

	// Add watermarks to poster and thumbnail
	if p.config.Watermark.Switch {
		fullPosterPath := filepath.Join(outputPath, posterPath)
		fullThumbPath := filepath.Join(outputPath, thumbPath)
		logger.Debug("Adding watermarks to: poster=%s, thumb=%s", fullPosterPath, fullThumbPath)
		err = p.watermark.AddWatermarks(fullPosterPath, fullThumbPath, flags.ChineseSubtitle, flags.Leak, uncensored, flags.Hack, flags.FourK, flags.ISO)
		if err != nil {
			logger.Warn("Failed to add watermarks: %v", err)
		} else {
			logger.Info("Successfully added watermarks")
		}
	}

	// Move/link the video file(s)
	if isMultiPart && fragmentGroup != nil {
		// For fragment groups, move all fragment files to the same directory
		logger.Info("Moving %d fragment files to output directory", totalParts)
		
		// Move all fragment files to the same directory with clean naming
		for i, fragInfo := range fragmentGroup.Fragments {
			// Skip if source file doesn't exist (already moved or missing)
			if _, err := os.Stat(fragInfo.FilePath); os.IsNotExist(err) {
				logger.Debug("Fragment file already moved or missing: %s", fragInfo.FilePath)
				continue
			}
			
			// Generate filename without fragment suffix for cleaner naming
			baseNumber := data.Number
			fragExt := filepath.Ext(fragInfo.FilePath)
			
			// Create sequential naming: movie-1.ext, movie-2.ext, etc.
			destFileName := fmt.Sprintf("%s-%d%s", baseNumber, i+1, fragExt)
			if flags.Leak {
				destFileName = fmt.Sprintf("%s-leak-%d%s", baseNumber, i+1, fragExt)
			}
			if flags.ChineseSubtitle && !flags.Hack && !flags.Leak {
				destFileName = fmt.Sprintf("%s-C-%d%s", baseNumber, i+1, fragExt)
			}
			if flags.Hack {
				destFileName = fmt.Sprintf("%s-hack-%d%s", baseNumber, i+1, fragExt)
			}
			
			destPath := filepath.Join(outputPath, destFileName)
			
			// Skip if destination file already exists
			if _, err := os.Stat(destPath); err == nil {
				logger.Debug("Fragment destination already exists, skipping: %s", destPath)
				continue
			}
			
			logger.Debug("Moving fragment %d: %s -> %s", i+1, fragInfo.FilePath, destPath)
			
			err = p.storage.MoveFile(fragInfo.FilePath, destPath)
			if err != nil {
				logger.Warn("Failed to move fragment file %s: %v", fragInfo.FilePath, err)
				// Continue with other files even if one fails
			} else {
				logger.Info("Successfully moved fragment %d: %s", i+1, filepath.Base(destPath))
			}
		}
	} else {
		// Single file processing
		destFileName := generateFileName(data.Number, flags.Part, flags.Leak, flags.ChineseSubtitle, flags.Hack, filepath.Ext(filePath))
		destPath := filepath.Join(outputPath, destFileName)
		err = p.storage.MoveFile(filePath, destPath)
		if err != nil {
			return fmt.Errorf("failed to move file: %w", err)
		}
	}

	// TODO: Move subtitle files

	// Generate NFO file with fragment information (do this last as completion marker)
	err = p.nfoGen.GenerateNFO(data, outputPath, flags.Part, flags.ChineseSubtitle, flags.Leak, uncensored, flags.Hack, flags.FourK, flags.ISO, data.ActorList, posterPath, thumbPath, fanartPath, isMultiPart, totalParts, currentPart, fragmentFiles, totalFileSize)
	if err != nil {
		return fmt.Errorf("failed to generate NFO: %w", err)
	}

	return nil
}

// processScrapingMode handles mode 1 (scraping with moving files)
func (p *Processor) processScrapingMode(ctx context.Context, filePath string, data *scraper.MovieData, part string, leak, chineseSubtitle, hack, fourK, iso, uncensored bool) error {
	// Create output folder
	outputPath, err := p.storage.CreateFolder(data)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	// Download images and generate file names
	ext := utils.GetImageExtension(data.Cover)
	var fanartPath, posterPath, thumbPath string
	
	if p.config.NameRule.ImageNamingWithNumber {
		// Use number-based naming
		leakWord := ""
		if leak {
			leakWord = "-leak"
		}
		cWord := ""
		if chineseSubtitle && !hack && !leak {
			cWord = "-C"
		}
		hackWord := ""
		if hack {
			hackWord = "-hack"
		}
		
		prefix := data.Number + leakWord + cWord + hackWord
		fanartPath = prefix + "-fanart" + ext
		posterPath = prefix + "-poster" + ext
		thumbPath = prefix + "-thumb" + ext
	} else {
		// Use simple naming
		fanartPath = "fanart" + ext
		posterPath = "poster" + ext
		thumbPath = "thumb" + ext
	}

	// Download cover image
	fullThumbPath := filepath.Join(outputPath, thumbPath)
	if data.Cover != "" {
		err = p.downloader.DownloadCover(ctx, data.Cover, fullThumbPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download cover: %v", err)
		} else {
			// Create fanart copy for non-Jellyfin
			if p.config.Common.Jellyfin == 0 {
				fullFanartPath := filepath.Join(outputPath, fanartPath)
				// Copy thumb to fanart (simplified, in real implementation you'd copy the file)
				p.downloader.DownloadCover(ctx, data.Cover, fullFanartPath, data.Headers)
			}
		}
	}

	// Download small cover if needed
	if data.ImageCut == 3 && data.CoverSmall != "" {
		smallCoverPath := filepath.Join(outputPath, posterPath)
		err = p.downloader.DownloadCover(ctx, data.CoverSmall, smallCoverPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download small cover: %v", err)
		}
	}

	// Download extra fanart (only for main part or single file)
	if (part == "" || strings.ToLower(part) == "-cd1") && p.config.Extrafanart.Switch && len(data.Extrafanart) > 0 {
		err = p.downloader.DownloadExtrafanart(ctx, data.Extrafanart, outputPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download extrafanart: %v", err)
		}
	}

	// Download trailer if enabled
	if (part == "" || strings.ToLower(part) == "-cd1") && p.config.Trailer.Switch && data.Trailer != "" {
		trailerName := fmt.Sprintf("%s%s-trailer.mp4", data.Number, getFileSuffix(leak, chineseSubtitle, hack))
		trailerPath := filepath.Join(outputPath, trailerName)
		err = p.downloader.DownloadTrailer(ctx, data.Trailer, trailerPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download trailer: %v", err)
		}
	}

	// Download actor photos if enabled
	if (part == "" || strings.ToLower(part) == "-cd1") && p.config.ActorPhoto.DownloadForKodi && len(data.ActorPhoto) > 0 {
		err = p.downloader.DownloadActorPhotos(ctx, data.ActorPhoto, outputPath)
		if err != nil {
			logger.Warn("Failed to download actor photos: %v", err)
		}
	}

	// Perform image cutting/cropping
	logger.Debug("Image cutting check: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	
	// Check if this is FC2 content - FC2 numbers don't need image cutting
	isFC2 := strings.HasPrefix(strings.ToUpper(data.Number), "FC2")
	if isFC2 {
		logger.Debug("Skipping image cutting for FC2 content: %s", data.Number)
		// For FC2, copy the same image to poster path (fanart, thumb, poster are the same)
		if fullThumbPath != "" && posterPath != "" {
			fullPosterPath := filepath.Join(outputPath, posterPath)
			err = p.imageProcessor.CopyImage(fullThumbPath, fullPosterPath)
			if err != nil {
				logger.Warn("Failed to copy image for FC2: %v", err)
			} else {
				logger.Info("Successfully copied image for FC2: %s -> %s", fullThumbPath, fullPosterPath)
			}
		}
	} else if data.ImageCut != 0 || p.config.Face.AlwaysImagecut {
		// Determine if we should skip face recognition
		skipFaceRec := p.config.Face.UncensoredOnly && !uncensored
		
		// Use imagecut from data, or 1 if always_imagecut is enabled
		imagecut := data.ImageCut
		if p.config.Face.AlwaysImagecut {
			imagecut = 1
		}
		
		logger.Debug("Performing image cutting: imagecut=%d, skipFaceRec=%v", imagecut, skipFaceRec)
		logger.Debug("Paths: fanart=%s, poster=%s", fullThumbPath, filepath.Join(outputPath, posterPath))
		
		// Only cut if we have both fanart and poster paths
		if fullThumbPath != "" && posterPath != "" {
			err = p.imageProcessor.CutImage(imagecut, fullThumbPath, filepath.Join(outputPath, posterPath), skipFaceRec)
			if err != nil {
				logger.Warn("Failed to cut image: %v", err)
			} else {
				logger.Info("Successfully cut image: %s -> %s", fullThumbPath, filepath.Join(outputPath, posterPath))
			}
		}
	} else {
		logger.Debug("Skipping image cutting: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	}

	// Add watermarks to poster and thumbnail
	if p.config.Watermark.Switch {
		fullPosterPath := filepath.Join(outputPath, posterPath)
		fullThumbPath := filepath.Join(outputPath, thumbPath)
		logger.Debug("Adding watermarks to: poster=%s, thumb=%s", fullPosterPath, fullThumbPath)
		err = p.watermark.AddWatermarks(fullPosterPath, fullThumbPath, chineseSubtitle, leak, uncensored, hack, fourK, iso)
		if err != nil {
			logger.Warn("Failed to add watermarks: %v", err)
		} else {
			logger.Info("Successfully added watermarks")
		}
	}

	// Move/link the video file
	destFileName := generateFileName(data.Number, part, leak, chineseSubtitle, hack, filepath.Ext(filePath))
	destPath := filepath.Join(outputPath, destFileName)
	err = p.storage.MoveFile(filePath, destPath)
	if err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	// TODO: Move subtitle files

	// Generate NFO file (do this last as completion marker)
	err = p.nfoGen.GenerateNFO(data, outputPath, part, chineseSubtitle, leak, uncensored, hack, fourK, iso, data.ActorList, posterPath, thumbPath, fanartPath, false, 0, 0, nil, 0)
	if err != nil {
		return fmt.Errorf("failed to generate NFO: %w", err)
	}

	return nil
}

// processOrganizingModeWithFragment handles mode 2 (organizing without scraping) with fragment support
func (p *Processor) processOrganizingModeWithFragment(filePath string, data *scraper.MovieData, flags utils.MovieFlags, isMultiPart bool, totalParts, currentPart int, fragmentFiles []string, totalFileSize int64, fragmentGroup *fragment.FragmentGroup) error {
	// Create output folder
	outputPath, err := p.storage.CreateFolder(data)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	// Move the file(s)
	if isMultiPart && fragmentGroup != nil {
		// For fragment groups, move all fragment files to the same directory
		logger.Info("Moving %d fragment files to output directory (organizing mode)", totalParts)
		
		// Move all fragment files to the same directory with clean naming
		for i, fragInfo := range fragmentGroup.Fragments {
			// Skip if source file doesn't exist (already moved or missing)
			if _, err := os.Stat(fragInfo.FilePath); os.IsNotExist(err) {
				logger.Debug("Fragment file already moved or missing: %s", fragInfo.FilePath)
				continue
			}
			
			// Generate filename without fragment suffix for cleaner naming
			baseNumber := data.Number
			fragExt := filepath.Ext(fragInfo.FilePath)
			
			// Create sequential naming: movie-1.ext, movie-2.ext, etc.
			destFileName := fmt.Sprintf("%s-%d%s", baseNumber, i+1, fragExt)
			if flags.Leak {
				destFileName = fmt.Sprintf("%s-leak-%d%s", baseNumber, i+1, fragExt)
			}
			if flags.ChineseSubtitle && !flags.Hack && !flags.Leak {
				destFileName = fmt.Sprintf("%s-C-%d%s", baseNumber, i+1, fragExt)
			}
			if flags.Hack {
				destFileName = fmt.Sprintf("%s-hack-%d%s", baseNumber, i+1, fragExt)
			}
			
			destPath := filepath.Join(outputPath, destFileName)
			
			// Skip if destination file already exists
			if _, err := os.Stat(destPath); err == nil {
				logger.Debug("Fragment destination already exists, skipping: %s", destPath)
				continue
			}
			
			logger.Debug("Moving fragment %d: %s -> %s", i+1, fragInfo.FilePath, destPath)
			
			err = p.storage.MoveFile(fragInfo.FilePath, destPath)
			if err != nil {
				logger.Warn("Failed to move fragment file %s: %v", fragInfo.FilePath, err)
				// Continue with other files even if one fails
			} else {
				logger.Info("Successfully moved fragment %d: %s", i+1, filepath.Base(destPath))
			}
		}
	} else {
		// Single file processing
		destFileName := generateFileName(data.Number, flags.Part, flags.Leak, flags.ChineseSubtitle, flags.Hack, filepath.Ext(filePath))
		destPath := filepath.Join(outputPath, destFileName)
		err = p.storage.MoveFile(filePath, destPath)
		if err != nil {
			return fmt.Errorf("failed to move file: %w", err)
		}
	}

	// TODO: Move subtitle files

	return nil
}

// processOrganizingMode handles mode 2 (organizing without scraping)
func (p *Processor) processOrganizingMode(filePath string, data *scraper.MovieData, part string, leak, chineseSubtitle, hack, fourK, iso bool) error {
	// Create output folder
	outputPath, err := p.storage.CreateFolder(data)
	if err != nil {
		return fmt.Errorf("failed to create folder: %w", err)
	}

	// Move the file
	destFileName := generateFileName(data.Number, part, leak, chineseSubtitle, hack, filepath.Ext(filePath))
	destPath := filepath.Join(outputPath, destFileName)
	err = p.storage.MoveFile(filePath, destPath)
	if err != nil {
		return fmt.Errorf("failed to move file: %w", err)
	}

	// TODO: Move subtitle files

	return nil
}

// processAnalysisModeWithFragment handles mode 3 (scraping in place) with fragment support
func (p *Processor) processAnalysisModeWithFragment(ctx context.Context, filePath string, data *scraper.MovieData, flags utils.MovieFlags, uncensored bool, isMultiPart bool, totalParts, currentPart int, fragmentFiles []string, totalFileSize int64, fragmentGroup *fragment.FragmentGroup) error {
	outputPath := filepath.Dir(filePath)

	// Generate file names (same logic as scraping mode)
	ext := utils.GetImageExtension(data.Cover)
	var fanartPath, posterPath, thumbPath string
	
	if p.config.NameRule.ImageNamingWithNumber {
		leakWord := ""
		if flags.Leak {
			leakWord = "-leak"
		}
		cWord := ""
		if flags.ChineseSubtitle && !flags.Hack && !flags.Leak {
			cWord = "-C"
		}
		hackWord := ""
		if flags.Hack {
			hackWord = "-hack"
		}
		
		prefix := data.Number + leakWord + cWord + hackWord
		fanartPath = prefix + "-fanart" + ext
		posterPath = prefix + "-poster" + ext
		thumbPath = prefix + "-thumb" + ext
	} else {
		fanartPath = "fanart" + ext
		posterPath = "poster" + ext
		thumbPath = "thumb" + ext
	}

	// Download images (same as scraping mode)
	if data.Cover != "" {
		fullThumbPath := filepath.Join(outputPath, thumbPath)
		err := p.downloader.DownloadCover(ctx, data.Cover, fullThumbPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download cover: %v", err)
		}

		if p.config.Common.Jellyfin == 0 {
			fullFanartPath := filepath.Join(outputPath, fanartPath)
			p.downloader.DownloadCover(ctx, data.Cover, fullFanartPath, data.Headers)
		}
	}

	// Perform image cutting/cropping (same logic as scraping mode)
	fullThumbPath := filepath.Join(outputPath, thumbPath)
	logger.Debug("Image cutting check: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	
	// Check if this is FC2 content - FC2 numbers don't need image cutting
	isFC2 := strings.HasPrefix(strings.ToUpper(data.Number), "FC2")
	if isFC2 {
		logger.Debug("Skipping image cutting for FC2 content: %s", data.Number)
		// For FC2, copy the same image to poster path (fanart, thumb, poster are the same)
		if fullThumbPath != "" && posterPath != "" {
			fullPosterPath := filepath.Join(outputPath, posterPath)
			err := p.imageProcessor.CopyImage(fullThumbPath, fullPosterPath)
			if err != nil {
				logger.Warn("Failed to copy image for FC2: %v", err)
			} else {
				logger.Info("Successfully copied image for FC2: %s -> %s", fullThumbPath, fullPosterPath)
			}
		}
	} else if data.ImageCut != 0 || p.config.Face.AlwaysImagecut {
		// Determine if we should skip face recognition
		skipFaceRec := p.config.Face.UncensoredOnly && !uncensored
		
		// Use imagecut from data, or 1 if always_imagecut is enabled
		imagecut := data.ImageCut
		if p.config.Face.AlwaysImagecut {
			imagecut = 1
		}
		
		logger.Debug("Performing image cutting: imagecut=%d, skipFaceRec=%v", imagecut, skipFaceRec)
		logger.Debug("Paths: fanart=%s, poster=%s", fullThumbPath, filepath.Join(outputPath, posterPath))
		
		// Only cut if we have both fanart and poster paths
		if fullThumbPath != "" && posterPath != "" {
			err := p.imageProcessor.CutImage(imagecut, fullThumbPath, filepath.Join(outputPath, posterPath), skipFaceRec)
			if err != nil {
				logger.Warn("Failed to cut image: %v", err)
			} else {
				logger.Info("Successfully cut image: %s -> %s", fullThumbPath, filepath.Join(outputPath, posterPath))
			}
		}
	} else {
		logger.Debug("Skipping image cutting: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	}

	// Add watermarks to poster and thumbnail (same logic as scraping mode)
	if p.config.Watermark.Switch {
		fullPosterPath := filepath.Join(outputPath, posterPath)
		fullThumbPath := filepath.Join(outputPath, thumbPath)
		logger.Debug("Adding watermarks to: poster=%s, thumb=%s", fullPosterPath, fullThumbPath)
		err := p.watermark.AddWatermarks(fullPosterPath, fullThumbPath, flags.ChineseSubtitle, flags.Leak, uncensored, flags.Hack, flags.FourK, flags.ISO)
		if err != nil {
			logger.Warn("Failed to add watermarks: %v", err)
		} else {
			logger.Info("Successfully added watermarks")
		}
	}

	// Download other resources (same logic as scraping mode)
	if (flags.Part == "" || strings.ToLower(flags.Part) == "-cd1") {
		// Extra fanart
		if p.config.Extrafanart.Switch && len(data.Extrafanart) > 0 {
			p.downloader.DownloadExtrafanart(ctx, data.Extrafanart, outputPath, data.Headers)
		}

		// Trailer
		if p.config.Trailer.Switch && data.Trailer != "" {
			trailerName := fmt.Sprintf("%s%s-trailer.mp4", data.Number, getFileSuffix(flags.Leak, flags.ChineseSubtitle, flags.Hack))
			trailerPath := filepath.Join(outputPath, trailerName)
			p.downloader.DownloadTrailer(ctx, data.Trailer, trailerPath, data.Headers)
		}

		// Actor photos
		if p.config.ActorPhoto.DownloadForKodi && len(data.ActorPhoto) > 0 {
			p.downloader.DownloadActorPhotos(ctx, data.ActorPhoto, outputPath)
		}
	}

	// Generate NFO with fragment information (filename must match video file exactly in mode 3)
	err := p.nfoGen.GenerateNFO(data, filePath, flags.Part, flags.ChineseSubtitle, flags.Leak, uncensored, flags.Hack, flags.FourK, flags.ISO, data.ActorList, posterPath, thumbPath, fanartPath, isMultiPart, totalParts, currentPart, fragmentFiles, totalFileSize)
	if err != nil {
		return fmt.Errorf("failed to generate NFO: %w", err)
	}

	return nil
}

// processAnalysisMode handles mode 3 (scraping in place)
func (p *Processor) processAnalysisMode(ctx context.Context, filePath string, data *scraper.MovieData, part string, leak, chineseSubtitle, hack, fourK, iso, uncensored bool) error {
	outputPath := filepath.Dir(filePath)

	// Generate file names (same logic as scraping mode)
	ext := utils.GetImageExtension(data.Cover)
	var fanartPath, posterPath, thumbPath string
	
	if p.config.NameRule.ImageNamingWithNumber {
		leakWord := ""
		if leak {
			leakWord = "-leak"
		}
		cWord := ""
		if chineseSubtitle && !hack && !leak {
			cWord = "-C"
		}
		hackWord := ""
		if hack {
			hackWord = "-hack"
		}
		
		prefix := data.Number + leakWord + cWord + hackWord
		fanartPath = prefix + "-fanart" + ext
		posterPath = prefix + "-poster" + ext
		thumbPath = prefix + "-thumb" + ext
	} else {
		fanartPath = "fanart" + ext
		posterPath = "poster" + ext
		thumbPath = "thumb" + ext
	}

	// Download images (same as scraping mode)
	if data.Cover != "" {
		fullThumbPath := filepath.Join(outputPath, thumbPath)
		err := p.downloader.DownloadCover(ctx, data.Cover, fullThumbPath, data.Headers)
		if err != nil {
			logger.Warn("Failed to download cover: %v", err)
		}

		if p.config.Common.Jellyfin == 0 {
			fullFanartPath := filepath.Join(outputPath, fanartPath)
			p.downloader.DownloadCover(ctx, data.Cover, fullFanartPath, data.Headers)
		}
	}

	// Perform image cutting/cropping (same logic as scraping mode)
	fullThumbPath := filepath.Join(outputPath, thumbPath)
	logger.Debug("Image cutting check: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	
	// Check if this is FC2 content - FC2 numbers don't need image cutting
	isFC2 := strings.HasPrefix(strings.ToUpper(data.Number), "FC2")
	if isFC2 {
		logger.Debug("Skipping image cutting for FC2 content: %s", data.Number)
		// For FC2, copy the same image to poster path (fanart, thumb, poster are the same)
		if fullThumbPath != "" && posterPath != "" {
			fullPosterPath := filepath.Join(outputPath, posterPath)
			err := p.imageProcessor.CopyImage(fullThumbPath, fullPosterPath)
			if err != nil {
				logger.Warn("Failed to copy image for FC2: %v", err)
			} else {
				logger.Info("Successfully copied image for FC2: %s -> %s", fullThumbPath, fullPosterPath)
			}
		}
	} else if data.ImageCut != 0 || p.config.Face.AlwaysImagecut {
		// Determine if we should skip face recognition
		skipFaceRec := p.config.Face.UncensoredOnly && !uncensored
		
		// Use imagecut from data, or 1 if always_imagecut is enabled
		imagecut := data.ImageCut
		if p.config.Face.AlwaysImagecut {
			imagecut = 1
		}
		
		logger.Debug("Performing image cutting: imagecut=%d, skipFaceRec=%v", imagecut, skipFaceRec)
		logger.Debug("Paths: fanart=%s, poster=%s", fullThumbPath, filepath.Join(outputPath, posterPath))
		
		// Only cut if we have both fanart and poster paths
		if fullThumbPath != "" && posterPath != "" {
			err := p.imageProcessor.CutImage(imagecut, fullThumbPath, filepath.Join(outputPath, posterPath), skipFaceRec)
			if err != nil {
				logger.Warn("Failed to cut image: %v", err)
			} else {
				logger.Info("Successfully cut image: %s -> %s", fullThumbPath, filepath.Join(outputPath, posterPath))
			}
		}
	} else {
		logger.Debug("Skipping image cutting: ImageCut=%d, AlwaysImagecut=%v", data.ImageCut, p.config.Face.AlwaysImagecut)
	}

	// Add watermarks to poster and thumbnail (same logic as scraping mode)
	if p.config.Watermark.Switch {
		fullPosterPath := filepath.Join(outputPath, posterPath)
		fullThumbPath := filepath.Join(outputPath, thumbPath)
		logger.Debug("Adding watermarks to: poster=%s, thumb=%s", fullPosterPath, fullThumbPath)
		err := p.watermark.AddWatermarks(fullPosterPath, fullThumbPath, chineseSubtitle, leak, uncensored, hack, fourK, iso)
		if err != nil {
			logger.Warn("Failed to add watermarks: %v", err)
		} else {
			logger.Info("Successfully added watermarks")
		}
	}

	// Download other resources (same logic as scraping mode)
	if (part == "" || strings.ToLower(part) == "-cd1") {
		// Extra fanart
		if p.config.Extrafanart.Switch && len(data.Extrafanart) > 0 {
			p.downloader.DownloadExtrafanart(ctx, data.Extrafanart, outputPath, data.Headers)
		}

		// Trailer
		if p.config.Trailer.Switch && data.Trailer != "" {
			trailerName := fmt.Sprintf("%s%s-trailer.mp4", data.Number, getFileSuffix(leak, chineseSubtitle, hack))
			trailerPath := filepath.Join(outputPath, trailerName)
			p.downloader.DownloadTrailer(ctx, data.Trailer, trailerPath, data.Headers)
		}

		// Actor photos
		if p.config.ActorPhoto.DownloadForKodi && len(data.ActorPhoto) > 0 {
			p.downloader.DownloadActorPhotos(ctx, data.ActorPhoto, outputPath)
		}
	}

	// Generate NFO (filename must match video file exactly in mode 3)
	err := p.nfoGen.GenerateNFO(data, filePath, part, chineseSubtitle, leak, uncensored, hack, fourK, iso, data.ActorList, posterPath, thumbPath, fanartPath, false, 0, 0, nil, 0)
	if err != nil {
		return fmt.Errorf("failed to generate NFO: %w", err)
	}

	return nil
}

// handleFailedFile handles files that failed processing
func (p *Processor) handleFailedFile(filePath string) {
	err := p.storage.MoveToFailedFolder(filePath)
	if err != nil {
		logger.Error("Failed to handle failed file %s: %v", filePath, err)
	}
}

// cleanupEmptyFolders removes empty directories
func (p *Processor) cleanupEmptyFolders() {
	if p.config.Common.SuccessOutputFolder != "" {
		err := p.storage.RemoveEmptyFolders(p.config.Common.SuccessOutputFolder)
		if err != nil {
			logger.Warn("Failed to cleanup success folder: %v", err)
		}
	}

	if p.config.Common.FailedOutputFolder != "" {
		err := p.storage.RemoveEmptyFolders(p.config.Common.FailedOutputFolder)
		if err != nil {
			logger.Warn("Failed to cleanup failed folder: %v", err)
		}
	}

	if p.config.Common.SourceFolder != "" {
		err := p.storage.RemoveEmptyFolders(p.config.Common.SourceFolder)
		if err != nil {
			logger.Warn("Failed to cleanup source folder: %v", err)
		}
	}
}

// generateFileName generates the destination filename
func generateFileName(number, part string, leak, chineseSubtitle, hack bool, ext string) string {
	leakWord := ""
	if leak {
		leakWord = "-leak"
	}
	
	cWord := ""
	if chineseSubtitle && !hack && !leak {
		cWord = "-C"
	}
	
	hackWord := ""
	if hack {
		hackWord = "-hack"
	}
	
	return number + part + leakWord + cWord + hackWord + ext
}

// getFileSuffix returns file suffix based on flags
func getFileSuffix(leak, chineseSubtitle, hack bool) string {
	leakWord := ""
	if leak {
		leakWord = "-leak"
	}
	
	cWord := ""
	if chineseSubtitle && !hack && !leak {
		cWord = "-C"
	}
	
	hackWord := ""
	if hack {
		hackWord = "-hack"
	}
	
	return leakWord + cWord + hackWord
}

// Close cleans up processor resources
func (p *Processor) Close() error {
	var errs []error

	if p.scraper != nil {
		if err := p.scraper.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if p.downloader != nil {
		if err := p.downloader.Close(); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("errors during cleanup: %v", errs)
	}

	return nil
}