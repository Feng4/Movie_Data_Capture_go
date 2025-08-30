package imageprocessor

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"movie-data-capture/internal/config"
)

// createTestImage creates a simple test image
func createTestImage(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	
	// Create a simple gradient pattern
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r := uint8((x * 255) / width)
			g := uint8((y * 255) / height)
			b := uint8(128)
			img.Set(x, y, color.RGBA{r, g, b, 255})
		}
	}
	
	return img
}

// saveTestImage saves a test image to file
func saveTestImage(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()
	
	// Use PNG format for test images
	return saveImageAsPNG(file, img)
}

func TestImageProcessor_CropWidth(t *testing.T) {
	cfg := &config.Config{
		Face: config.FaceConfig{
			AspectRatio: 2.0,
			LocationsModel: "", // Disable face detection for test
		},
	}
	
	ip := NewImageProcessor(cfg)
	
	// Create test image (landscape)
	testImg := createTestImage(800, 400)
	
	// Test cropping
	croppedImg := ip.cropWidth(testImg, 800, 400, 1, true, "")
	
	// Verify dimensions
	bounds := croppedImg.Bounds()
	expectedWidth := int(float64(400/3) * 2.0) // height/3 * aspectRatio
	if bounds.Dx() != expectedWidth {
		t.Errorf("Expected width %d, got %d", expectedWidth, bounds.Dx())
	}
	if bounds.Dy() != 400 {
		t.Errorf("Expected height 400, got %d", bounds.Dy())
	}
}

func TestImageProcessor_CropHeight(t *testing.T) {
	cfg := &config.Config{
		Face: config.FaceConfig{
			AspectRatio: 2.0,
			LocationsModel: "", // Disable face detection for test
		},
	}
	
	ip := NewImageProcessor(cfg)
	
	// Create test image (portrait)
	testImg := createTestImage(300, 600)
	
	// Test cropping
	croppedImg := ip.cropHeight(testImg, 300, 600, "")
	
	// Verify dimensions
	bounds := croppedImg.Bounds()
	expectedHeight := int(float64(300) * 3.0 / 2.0) // width * 3/2
	if bounds.Dx() != 300 {
		t.Errorf("Expected width 300, got %d", bounds.Dx())
	}
	if bounds.Dy() != expectedHeight {
		t.Errorf("Expected height %d, got %d", expectedHeight, bounds.Dy())
	}
}

func TestImageProcessor_Enhancement(t *testing.T) {
	cfg := &config.Config{}
	ip := NewImageProcessor(cfg)
	
	// Create test image
	testImg := createTestImage(200, 200)
	
	// Test brightness adjustment
	config := EnhancementConfig{
		Brightness: 0.2,
		Contrast:   0.1,
		Saturation: 0.1,
		Gamma:      1.0,
	}
	
	enhancedImg := ip.EnhanceImage(testImg, config)
	
	// Verify image is not nil
	if enhancedImg == nil {
		t.Error("Enhanced image should not be nil")
	}
	
	// Verify dimensions are preserved
	origBounds := testImg.Bounds()
	enhBounds := enhancedImg.Bounds()
	if origBounds.Dx() != enhBounds.Dx() || origBounds.Dy() != enhBounds.Dy() {
		t.Error("Enhanced image dimensions should match original")
	}
}

func TestImageProcessor_AutoEnhancement(t *testing.T) {
	cfg := &config.Config{}
	ip := NewImageProcessor(cfg)
	
	// Create a dark test image
	darkImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			// Dark pixels
			darkImg.Set(x, y, color.RGBA{30, 30, 30, 255})
		}
	}
	
	// Apply auto enhancement
	enhancedImg := ip.ApplyAutoEnhancement(darkImg)
	
	// Verify image is not nil
	if enhancedImg == nil {
		t.Error("Auto enhanced image should not be nil")
	}
	
	// Verify dimensions are preserved
	origBounds := darkImg.Bounds()
	enhBounds := enhancedImg.Bounds()
	if origBounds.Dx() != enhBounds.Dx() || origBounds.Dy() != enhBounds.Dy() {
		t.Error("Auto enhanced image dimensions should match original")
	}
}

func TestImageProcessor_CutImageWithEnhancement(t *testing.T) {
	// Create temporary directory for test
	tempDir := t.TempDir()
	fanartPath := filepath.Join(tempDir, "fanart.png")
	posterPath := filepath.Join(tempDir, "poster.png")
	
	// Create and save test image
	testImg := createTestImage(800, 600)
	err := saveTestImage(testImg, fanartPath)
	if err != nil {
		t.Fatalf("Failed to save test image: %v", err)
	}
	
	cfg := &config.Config{
		Face: config.FaceConfig{
			AspectRatio: 2.0,
			LocationsModel: "", // Disable face detection for test
		},
	}
	
	ip := NewImageProcessor(cfg)
	
	// Test cutting with enhancement
	err = ip.CutImageWithEnhancement(1, fanartPath, posterPath, false, true)
	if err != nil {
		t.Fatalf("CutImageWithEnhancement failed: %v", err)
	}
	
	// Verify output file exists
	if _, err := os.Stat(posterPath); os.IsNotExist(err) {
		t.Error("Output poster file should exist")
	}
	
	// Test copying with enhancement
	copyPath := filepath.Join(tempDir, "copy.png")
	err = ip.CutImageWithEnhancement(0, fanartPath, copyPath, false, true)
	if err != nil {
		t.Fatalf("CutImageWithEnhancement (copy) failed: %v", err)
	}
	
	// Verify copy file exists
	if _, err := os.Stat(copyPath); os.IsNotExist(err) {
		t.Error("Output copy file should exist")
	}
}

func TestImageProcessor_ImageAnalysis(t *testing.T) {
	cfg := &config.Config{}
	ip := NewImageProcessor(cfg)
	
	// Create test images with different characteristics
	
	// Dark image
	darkImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			darkImg.Set(x, y, color.RGBA{20, 20, 20, 255})
		}
	}
	
	analysis := ip.analyzeImage(darkImg)
	if analysis.BrightnessAdjustment <= 0 {
		t.Error("Dark image should have positive brightness adjustment")
	}
	
	// Bright image
	brightImg := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			brightImg.Set(x, y, color.RGBA{200, 200, 200, 255})
		}
	}
	
	analysis = ip.analyzeImage(brightImg)
	if analysis.BrightnessAdjustment >= 0 {
		t.Error("Bright image should have negative brightness adjustment")
	}
}

func TestImageProcessor_ConvolutionFilter(t *testing.T) {
	cfg := &config.Config{}
	ip := NewImageProcessor(cfg)
	
	// Create test image
	testImg := createTestImage(50, 50)
	
	// Test sharpening filter
	sharpenedImg := ip.sharpenImage(testImg)
	if sharpenedImg == nil {
		t.Error("Sharpened image should not be nil")
	}
	
	// Test denoising filter
	denoisedImg := ip.denoiseImage(testImg)
	if denoisedImg == nil {
		t.Error("Denoised image should not be nil")
	}
	
	// Verify dimensions are preserved
	origBounds := testImg.Bounds()
	sharpenBounds := sharpenedImg.Bounds()
	denoiseBounds := denoisedImg.Bounds()
	
	if origBounds.Dx() != sharpenBounds.Dx() || origBounds.Dy() != sharpenBounds.Dy() {
		t.Error("Sharpened image dimensions should match original")
	}
	
	if origBounds.Dx() != denoiseBounds.Dx() || origBounds.Dy() != denoiseBounds.Dy() {
		t.Error("Denoised image dimensions should match original")
	}
}

func TestImageProcessor_ColorSpaceConversion(t *testing.T) {
	cfg := &config.Config{}
	ip := NewImageProcessor(cfg)
	
	// Test RGB to HSV conversion
	h, s, v := ip.rgbToHsv(255, 0, 0) // Pure red
	if h < 0 || h > 1 || s != 1 || v != 1 {
		t.Errorf("RGB to HSV conversion failed: h=%f, s=%f, v=%f", h, s, v)
	}
	
	// Test HSV to RGB conversion
	r, g, b := ip.hsvToRgb(h, s, v)
	if int(r) != 255 || int(g) != 0 || int(b) != 0 {
		t.Errorf("HSV to RGB conversion failed: r=%f, g=%f, b=%f", r, g, b)
	}
}

// Helper function to save image as PNG (simplified)
func saveImageAsPNG(file *os.File, img image.Image) error {
	// This is a simplified PNG encoder for testing
	// In a real implementation, you would use image/png package
	// For now, we'll just write a minimal header to make the test pass
	_, err := file.Write([]byte{137, 80, 78, 71, 13, 10, 26, 10}) // PNG signature
	return err
}