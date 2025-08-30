package imageprocessor

import (
	"fmt"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/facedetection"
	"movie-data-capture/pkg/logger"
)

// ImageProcessor handles image cutting and processing operations
type ImageProcessor struct {
	config       *config.Config
	faceDetector *facedetection.FaceDetector
}

// NewImageProcessor creates a new image processor
func NewImageProcessor(cfg *config.Config) *ImageProcessor {
	// Initialize face detector with model path from config
	modelPath := ""
	if cfg.Face.LocationsModel != "" {
		modelPath = cfg.Face.LocationsModel
	}
	
	return &ImageProcessor{
		config:       cfg,
		faceDetector: facedetection.NewFaceDetector(modelPath),
	}
}

// CutImage performs image cutting based on imagecut parameter
// imagecut: 0=copy, 1=crop with face detection, 4=crop with face detection for uncensored
func (ip *ImageProcessor) CutImage(imagecut int, fanartPath, posterPath string, skipFaceRec bool) error {
	if imagecut == 0 {
		// Copy fanart to poster
		return ip.copyImage(fanartPath, posterPath)
	}

	if imagecut == 1 || imagecut == 4 {
		// Crop image
		return ip.cropImage(fanartPath, posterPath, imagecut, skipFaceRec)
	}

	return nil
}

// copyImageWithEnhancement copies and enhances the image
func (ip *ImageProcessor) copyImageWithEnhancement(srcPath, dstPath string) error {
	// Open source image
	img, err := ip.openImage(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}

	// Apply auto enhancement
	enhancedImg := ip.ApplyAutoEnhancement(img)

	// Save enhanced image
	err = ip.saveImage(enhancedImg, dstPath)
	if err != nil {
		return fmt.Errorf("failed to save enhanced image: %w", err)
	}

	logger.Info("[+]Image Enhanced & Copied! %s", filepath.Base(dstPath))
	return nil
}

// cropImageWithEnhancement crops and enhances the image
func (ip *ImageProcessor) cropImageWithEnhancement(srcPath, dstPath string, imagecut int, skipFaceRec bool, enhance bool) error {
	// Open source image
	img, err := ip.openImage(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate aspect ratio
	aspectRatio := float64(width) / float64(height)
	targetRatio := 2.0 / 3.0 // Target aspect ratio

	var croppedImg image.Image

	if aspectRatio > targetRatio {
		// Image is too wide, crop horizontally
		croppedImg = ip.cropWidth(img, width, height, imagecut, skipFaceRec, srcPath)
	} else if aspectRatio < targetRatio {
		// Image is too tall, crop vertically
		croppedImg = ip.cropHeight(img, width, height, srcPath)
	} else {
		// Image already has correct aspect ratio
		croppedImg = img
	}

	// Apply enhancement if requested
	if enhance {
		croppedImg = ip.ApplyAutoEnhancement(croppedImg)
	}

	// Save cropped and enhanced image
	err = ip.saveImage(croppedImg, dstPath)
	if err != nil {
		return fmt.Errorf("failed to save cropped image: %w", err)
	}

	if enhance {
		logger.Info("[+]Image Enhanced & Cutted! %s", filepath.Base(dstPath))
	} else {
		logger.Info("[+]Image Cutted! %s", filepath.Base(dstPath))
	}
	return nil
}

// CutImageWithEnhancement performs image cutting with optional enhancement
func (ip *ImageProcessor) CutImageWithEnhancement(imagecut int, fanartPath, posterPath string, skipFaceRec bool, enhance bool) error {
	if imagecut == 0 {
		// Copy fanart to poster with optional enhancement
		if enhance {
			return ip.copyImageWithEnhancement(fanartPath, posterPath)
		}
		return ip.copyImage(fanartPath, posterPath)
	}

	if imagecut == 1 || imagecut == 4 {
		// Crop image with optional enhancement
		return ip.cropImageWithEnhancement(fanartPath, posterPath, imagecut, skipFaceRec, enhance)
	}

	return nil
}

// CopyImage is a public method to copy image from source to destination
func (ip *ImageProcessor) CopyImage(srcPath, dstPath string) error {
	return ip.copyImage(srcPath, dstPath)
}

// copyImage copies the fanart image to poster path
func (ip *ImageProcessor) copyImage(srcPath, dstPath string) error {
	srcFile, err := os.Open(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open source image: %w", err)
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dstPath)
	if err != nil {
		return fmt.Errorf("failed to create destination image: %w", err)
	}
	defer dstFile.Close()

	// Copy file content
	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		return fmt.Errorf("failed to copy image: %w", err)
	}

	logger.Info("[+]Image Copied!     %s", filepath.Base(dstPath))
	return nil
}

// cropImage crops the image based on aspect ratio and face detection
func (ip *ImageProcessor) cropImage(srcPath, dstPath string, imagecut int, skipFaceRec bool) error {
	// Open source image
	img, err := ip.openImage(srcPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// Calculate aspect ratio
	aspectRatio := float64(width) / float64(height)
	targetRatio := 2.0 / 3.0 // Target aspect ratio

	var croppedImg image.Image

	if aspectRatio > targetRatio {
		// Image is too wide, crop horizontally
		croppedImg = ip.cropWidth(img, width, height, imagecut, skipFaceRec, srcPath)
	} else if aspectRatio < targetRatio {
		// Image is too tall, crop vertically
		croppedImg = ip.cropHeight(img, width, height, srcPath)
	} else {
		// Image already has correct aspect ratio
		croppedImg = img
	}

	// Save cropped image
	err = ip.saveImage(croppedImg, dstPath)
	if err != nil {
		return fmt.Errorf("failed to save cropped image: %w", err)
	}

	logger.Info("[+]Image Cutted!     %s", filepath.Base(dstPath))
	return nil
}

// cropWidth crops image horizontally
func (ip *ImageProcessor) cropWidth(img image.Image, width, height, imagecut int, skipFaceRec bool, srcPath string) image.Image {
	aspectRatio := ip.config.Face.AspectRatio
	cropWidthHalf := int(float64(height) / 3.0)
	newWidth := int(float64(cropWidthHalf) * aspectRatio)

	var cropLeft, cropRight int

	if imagecut == 4 || (!skipFaceRec && imagecut == 1) {
		// Try face detection
		centerX, _, found := ip.faceDetector.DetectFaces(srcPath)
		
		if found {
			// Use detected face center for cropping
			cropLeft = centerX - newWidth/2
			cropRight = centerX + newWidth/2
			
			// Boundary check
			if cropLeft < 0 {
				cropLeft = 0
				cropRight = newWidth
			} else if cropRight > width {
				cropLeft = width - newWidth
				cropRight = width
			}
		} else {
			// Fallback: use center-based cropping
			center := width / 2
			cropLeft = center - newWidth/2
			cropRight = center + newWidth/2
			
			// Boundary check
			if cropLeft < 0 {
				cropLeft = 0
				cropRight = newWidth
			} else if cropRight > width {
				cropLeft = width - newWidth
				cropRight = width
			}
		}
	} else {
		// Default: crop from right side (for censored content)
		cropLeft = width - newWidth
		cropRight = width
	}

	// Create sub-image
	cropRect := image.Rect(cropLeft, 0, cropRight, height)
	return ip.cropImageRect(img, cropRect)
}

// cropHeight crops image vertically with face detection support
func (ip *ImageProcessor) cropHeight(img image.Image, width, height int, srcPath string) image.Image {
	// Calculate new height to maintain 2:3 aspect ratio
	newHeight := int(float64(width) * 3.0 / 2.0)
	if newHeight > height {
		newHeight = height
	}

	var cropTop int
	
	// Try face detection for better vertical positioning
	_, topY, found := ip.faceDetector.DetectFaces(srcPath)
	
	if found {
		// Position crop area to include the detected face
		// Keep face in upper portion of the cropped area
		cropTop = topY
		cropBottom := cropTop + newHeight
		
		// Boundary check
		if cropBottom > height {
			cropTop = height - newHeight
			if cropTop < 0 {
				cropTop = 0
			}
		}
	} else {
		// Fallback: crop from bottom (original behavior)
		cropTop = height - newHeight
		if cropTop < 0 {
			cropTop = 0
		}
	}

	cropRect := image.Rect(0, cropTop, width, cropTop+newHeight)
	return ip.cropImageRect(img, cropRect)
}

// cropImageRect crops image to specified rectangle
func (ip *ImageProcessor) cropImageRect(img image.Image, rect image.Rectangle) image.Image {
	if subImg, ok := img.(interface {
		SubImage(r image.Rectangle) image.Image
	}); ok {
		return subImg.SubImage(rect)
	}

	// Fallback: create new image and copy pixels
	bounds := rect
	newImg := image.NewRGBA(image.Rect(0, 0, bounds.Dx(), bounds.Dy()))
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			newImg.Set(x-bounds.Min.X, y-bounds.Min.Y, img.At(x, y))
		}
	}
	return newImg
}

// openImage opens an image file
func (ip *ImageProcessor) openImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

// saveImage saves an image to file
func (ip *ImageProcessor) saveImage(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// Determine format based on file extension
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
	case ".png":
		return png.Encode(file, img)
	default:
		// Default to JPEG
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
	}
}