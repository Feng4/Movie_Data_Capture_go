package downloader

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"

	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/httpclient"
	"movie-data-capture/pkg/logger"
)

// Downloader handles file downloads with parallel support
type Downloader struct {
	config     *config.Config
	httpClient *httpclient.Client
}

// DownloadTask represents a download task
type DownloadTask struct {
	URL      string
	FilePath string
	Headers  map[string]string
}

// DownloadResult represents the result of a download
type DownloadResult struct {
	Task     DownloadTask
	Success  bool
	Error    error
	FilePath string
}

// New creates a new downloader instance
func New(cfg *config.Config) *Downloader {
	return &Downloader{
		config:     cfg,
		httpClient: httpclient.NewClient(&cfg.Proxy),
	}
}

// DownloadFile downloads a single file
func (d *Downloader) DownloadFile(ctx context.Context, url, filePath string, headers map[string]string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Check if file already exists and we should skip
	if d.config.Common.DownloadOnlyMissingImages {
		if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
			logger.Debug("File already exists, skipping: %s", filePath)
			return nil
		}
	}

	// Download the file
	resp, err := d.httpClient.Get(ctx, url, headers)
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", url, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("download failed with status %d: %s", resp.StatusCode, url)
	}

	// Create the file
	file, err := os.Create(filePath)
	if err != nil {
		return fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer file.Close()

	// Copy data
	_, err = io.Copy(file, resp.Body)
	if err != nil {
		// Remove partially downloaded file
		os.Remove(filePath)
		return fmt.Errorf("failed to write file %s: %w", filePath, err)
	}

	logger.Info("Downloaded: %s", filepath.Base(filePath))
	return nil
}

// DownloadFiles downloads multiple files in parallel
func (d *Downloader) DownloadFiles(ctx context.Context, tasks []DownloadTask) []DownloadResult {
	if len(tasks) == 0 {
		return nil
	}

	// Determine number of workers
	maxWorkers := d.config.Extrafanart.ParallelDownload
	if maxWorkers <= 0 {
		maxWorkers = 5
	}
	if maxWorkers > len(tasks) {
		maxWorkers = len(tasks)
	}

	// Warn about too many parallel downloads
	if maxWorkers > 100 {
		logger.Warn("Parallel download thread too large (%d) may cause website ban IP!", maxWorkers)
	}

	// Create channels
	taskChan := make(chan DownloadTask, len(tasks))
	resultChan := make(chan DownloadResult, len(tasks))

	// Start workers
	var wg sync.WaitGroup
	for i := 0; i < maxWorkers; i++ {
		wg.Add(1)
		go d.downloadWorker(ctx, &wg, taskChan, resultChan)
	}

	// Send tasks
	for _, task := range tasks {
		taskChan <- task
	}
	close(taskChan)

	// Wait for completion
	go func() {
		wg.Wait()
		close(resultChan)
	}()

	// Collect results
	var results []DownloadResult
	for result := range resultChan {
		results = append(results, result)
	}

	return results
}

// downloadWorker is a worker goroutine for downloading files
func (d *Downloader) downloadWorker(ctx context.Context, wg *sync.WaitGroup, taskChan <-chan DownloadTask, resultChan chan<- DownloadResult) {
	defer wg.Done()

	for task := range taskChan {
		result := DownloadResult{
			Task: task,
		}

		err := d.DownloadFile(ctx, task.URL, task.FilePath, task.Headers)
		if err != nil {
			result.Error = err
			result.Success = false
			logger.Error("Download failed: %s -> %s: %v", task.URL, task.FilePath, err)
		} else {
			result.Success = true
			result.FilePath = task.FilePath
		}

		resultChan <- result
	}
}

// DownloadCover downloads movie cover image
func (d *Downloader) DownloadCover(ctx context.Context, url, savePath string, headers map[string]string) error {
	if url == "" {
		return fmt.Errorf("cover URL is empty")
	}

	return d.DownloadFile(ctx, url, savePath, headers)
}

// DownloadExtrafanart downloads extra fanart images
func (d *Downloader) DownloadExtrafanart(ctx context.Context, urls []string, saveDir string, headers map[string]string) error {
	if len(urls) == 0 {
		return nil
	}

	// Create extrafanart directory
	extrafanartDir := filepath.Join(saveDir, d.config.Extrafanart.ExtrafanartFolder)
	if err := os.MkdirAll(extrafanartDir, 0755); err != nil {
		return fmt.Errorf("failed to create extrafanart directory: %w", err)
	}

	// Create download tasks
	var tasks []DownloadTask
	for i, url := range urls {
		if url == "" {
			continue
		}

		filename := fmt.Sprintf("extrafanart-%d.jpg", i+1)
		filePath := filepath.Join(extrafanartDir, filename)

		// Skip if file exists and we should only download missing
		if d.config.Common.DownloadOnlyMissingImages {
			if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
				continue
			}
		}

		tasks = append(tasks, DownloadTask{
			URL:      url,
			FilePath: filePath,
			Headers:  headers,
		})
	}

	if len(tasks) == 0 {
		logger.Debug("No extrafanart images to download")
		return nil
	}

	// Download in parallel
	results := d.DownloadFiles(ctx, tasks)

	// Count successes and failures
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	if failureCount > 0 {
		logger.Warn("Failed to download %d/%d extrafanart images", failureCount, len(results))
	} else {
		logger.Info("Successfully downloaded %d extrafanart images", successCount)
	}

	return nil
}

// DownloadActorPhotos downloads actor photos
func (d *Downloader) DownloadActorPhotos(ctx context.Context, actorPhotos map[string]string, saveDir string) error {
	if len(actorPhotos) == 0 {
		return nil
	}

	// Create actors directory
	actorsDir := filepath.Join(saveDir, ".actors")
	if err := os.MkdirAll(actorsDir, 0755); err != nil {
		return fmt.Errorf("failed to create actors directory: %w", err)
	}

	// Create download tasks
	var tasks []DownloadTask
	for actorName, photoURL := range actorPhotos {
		if photoURL == "" {
			continue
		}

		// Determine file extension from URL
		ext := ".jpg"
		if filepath.Ext(photoURL) != "" {
			ext = filepath.Ext(photoURL)
		}

		filename := actorName + ext
		filePath := filepath.Join(actorsDir, filename)

		// Skip if file exists and we should only download missing
		if d.config.Common.DownloadOnlyMissingImages {
			if info, err := os.Stat(filePath); err == nil && info.Size() > 0 {
				continue
			}
		}

		tasks = append(tasks, DownloadTask{
			URL:      photoURL,
			FilePath: filePath,
		})
	}

	if len(tasks) == 0 {
		logger.Debug("No actor photos to download")
		return nil
	}

	// Download in parallel
	results := d.DownloadFiles(ctx, tasks)

	// Count successes and failures
	successCount := 0
	failureCount := 0
	for _, result := range results {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}
	}

	if failureCount > 0 {
		logger.Warn("Failed to download %d/%d actor photos", failureCount, len(results))
	} else {
		logger.Info("Successfully downloaded %d actor photos", successCount)
	}

	return nil
}

// DownloadTrailer downloads movie trailer
func (d *Downloader) DownloadTrailer(ctx context.Context, url, savePath string, headers map[string]string) error {
	if url == "" {
		return fmt.Errorf("trailer URL is empty")
	}

	return d.DownloadFile(ctx, url, savePath, headers)
}

// Close closes the downloader and cleans up resources
func (d *Downloader) Close() error {
	if d.httpClient != nil {
		return d.httpClient.Close()
	}
	return nil
}