package imageprocessor

import (
	"image"
	"image/color"
	"math"

	"movie-data-capture/pkg/logger"
)

// EnhancementConfig holds configuration for image enhancement
type EnhancementConfig struct {
	Sharpen    bool    // Enable sharpening
	Denoise    bool    // Enable denoising
	Contrast   float64 // Contrast adjustment (-1.0 to 1.0)
	Brightness float64 // Brightness adjustment (-1.0 to 1.0)
	Saturation float64 // Saturation adjustment (-1.0 to 1.0)
	Gamma      float64 // Gamma correction (0.1 to 3.0)
}

// DefaultEnhancementConfig returns default enhancement configuration
func DefaultEnhancementConfig() EnhancementConfig {
	return EnhancementConfig{
		Sharpen:    false,
		Denoise:    false,
		Contrast:   0.0,
		Brightness: 0.0,
		Saturation: 0.0,
		Gamma:      1.0,
	}
}

// EnhanceImage applies various enhancement filters to the image
func (ip *ImageProcessor) EnhanceImage(img image.Image, config EnhancementConfig) image.Image {
	result := img

	// Apply enhancements in order
	if config.Brightness != 0.0 || config.Contrast != 0.0 || config.Saturation != 0.0 {
		result = ip.adjustBrightnessContrastSaturation(result, config.Brightness, config.Contrast, config.Saturation)
	}

	if config.Gamma != 1.0 {
		result = ip.adjustGamma(result, config.Gamma)
	}

	if config.Denoise {
		result = ip.denoiseImage(result)
	}

	if config.Sharpen {
		result = ip.sharpenImage(result)
	}

	logger.Debug("Image enhancement applied")
	return result
}

// adjustBrightnessContrastSaturation adjusts brightness, contrast, and saturation
func (ip *ImageProcessor) adjustBrightnessContrastSaturation(img image.Image, brightness, contrast, saturation float64) image.Image {
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)

	// Precompute contrast factor
	contrastFactor := (259.0 * (contrast*255.0 + 255.0)) / (255.0 * (259.0 - contrast*255.0))

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			original := img.At(x, y)
			r, g, b, a := original.RGBA()

			// Convert to 8-bit
			r8 := float64(r >> 8)
			g8 := float64(g >> 8)
			b8 := float64(b >> 8)
			a8 := uint8(a >> 8)

			// Apply brightness
			r8 += brightness * 255.0
			g8 += brightness * 255.0
			b8 += brightness * 255.0

			// Apply contrast
			r8 = contrastFactor*(r8-128.0) + 128.0
			g8 = contrastFactor*(g8-128.0) + 128.0
			b8 = contrastFactor*(b8-128.0) + 128.0

			// Apply saturation
			if saturation != 0.0 {
				// Convert to HSV, adjust saturation, convert back
				h, s, v := ip.rgbToHsv(r8, g8, b8)
				s = math.Max(0, math.Min(1, s+saturation))
				r8, g8, b8 = ip.hsvToRgb(h, s, v)
			}

			// Clamp values
			r8 = math.Max(0, math.Min(255, r8))
			g8 = math.Max(0, math.Min(255, g8))
			b8 = math.Max(0, math.Min(255, b8))

			result.Set(x, y, color.RGBA{
				uint8(r8),
				uint8(g8),
				uint8(b8),
				a8,
			})
		}
	}

	return result
}

// adjustGamma applies gamma correction
func (ip *ImageProcessor) adjustGamma(img image.Image, gamma float64) image.Image {
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)

	// Precompute gamma lookup table
	gammaLUT := make([]uint8, 256)
	for i := 0; i < 256; i++ {
		value := math.Pow(float64(i)/255.0, 1.0/gamma) * 255.0
		gammaLUT[i] = uint8(math.Max(0, math.Min(255, value)))
	}

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			original := img.At(x, y)
			r, g, b, a := original.RGBA()

			// Apply gamma correction using lookup table
			newR := gammaLUT[r>>8]
			newG := gammaLUT[g>>8]
			newB := gammaLUT[b>>8]

			result.Set(x, y, color.RGBA{newR, newG, newB, uint8(a >> 8)})
		}
	}

	return result
}

// sharpenImage applies sharpening filter
func (ip *ImageProcessor) sharpenImage(img image.Image) image.Image {
	// Sharpening kernel
	kernel := [][]float64{
		{0, -1, 0},
		{-1, 5, -1},
		{0, -1, 0},
	}

	return ip.applyConvolutionFilter(img, kernel)
}

// denoiseImage applies denoising filter (Gaussian blur)
func (ip *ImageProcessor) denoiseImage(img image.Image) image.Image {
	// Gaussian blur kernel for denoising
	kernel := [][]float64{
		{1.0 / 16, 2.0 / 16, 1.0 / 16},
		{2.0 / 16, 4.0 / 16, 2.0 / 16},
		{1.0 / 16, 2.0 / 16, 1.0 / 16},
	}

	return ip.applyConvolutionFilter(img, kernel)
}

// applyConvolutionFilter applies a convolution filter to the image
func (ip *ImageProcessor) applyConvolutionFilter(img image.Image, kernel [][]float64) image.Image {
	bounds := img.Bounds()
	result := image.NewRGBA(bounds)

	kernelSize := len(kernel)
	kernelRadius := kernelSize / 2

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			var sumR, sumG, sumB float64
			var sumA uint32

			// Apply kernel
			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					// Calculate source pixel coordinates
					srcX := x + kx - kernelRadius
					srcY := y + ky - kernelRadius

					// Handle edge cases by clamping coordinates
					if srcX < bounds.Min.X {
						srcX = bounds.Min.X
					} else if srcX >= bounds.Max.X {
						srcX = bounds.Max.X - 1
					}
					if srcY < bounds.Min.Y {
						srcY = bounds.Min.Y
					} else if srcY >= bounds.Max.Y {
						srcY = bounds.Max.Y - 1
					}

					// Get pixel value
					pixel := img.At(srcX, srcY)
					r, g, b, a := pixel.RGBA()

					// Apply kernel weight
					weight := kernel[ky][kx]
					sumR += float64(r>>8) * weight
					sumG += float64(g>>8) * weight
					sumB += float64(b>>8) * weight
					sumA = a // Keep original alpha
				}
			}

			// Clamp values and set result pixel
			newR := uint8(math.Max(0, math.Min(255, sumR)))
			newG := uint8(math.Max(0, math.Min(255, sumG)))
			newB := uint8(math.Max(0, math.Min(255, sumB)))
			newA := uint8(sumA >> 8)

			result.Set(x, y, color.RGBA{newR, newG, newB, newA})
		}
	}

	return result
}

// rgbToHsv converts RGB to HSV color space
func (ip *ImageProcessor) rgbToHsv(r, g, b float64) (h, s, v float64) {
	r /= 255.0
	g /= 255.0
	b /= 255.0

	max := math.Max(math.Max(r, g), b)
	min := math.Min(math.Min(r, g), b)
	delta := max - min

	// Value
	v = max

	// Saturation
	if max == 0 {
		s = 0
	} else {
		s = delta / max
	}

	// Hue
	if delta == 0 {
		h = 0
	} else if max == r {
		h = 60 * (((g - b) / delta) + 0)
	} else if max == g {
		h = 60 * (((b - r) / delta) + 2)
	} else {
		h = 60 * (((r - g) / delta) + 4)
	}

	if h < 0 {
		h += 360
	}

	h /= 360.0 // Normalize to 0-1
	return h, s, v
}

// hsvToRgb converts HSV to RGB color space
func (ip *ImageProcessor) hsvToRgb(h, s, v float64) (r, g, b float64) {
	h *= 360.0 // Convert back to 0-360

	c := v * s
	x := c * (1 - math.Abs(math.Mod(h/60.0, 2)-1))
	m := v - c

	var r1, g1, b1 float64

	if h >= 0 && h < 60 {
		r1, g1, b1 = c, x, 0
	} else if h >= 60 && h < 120 {
		r1, g1, b1 = x, c, 0
	} else if h >= 120 && h < 180 {
		r1, g1, b1 = 0, c, x
	} else if h >= 180 && h < 240 {
		r1, g1, b1 = 0, x, c
	} else if h >= 240 && h < 300 {
		r1, g1, b1 = x, 0, c
	} else {
		r1, g1, b1 = c, 0, x
	}

	r = (r1 + m) * 255.0
	g = (g1 + m) * 255.0
	b = (b1 + m) * 255.0

	return r, g, b
}

// ApplyAutoEnhancement applies automatic image enhancement based on image analysis
func (ip *ImageProcessor) ApplyAutoEnhancement(img image.Image) image.Image {
	// Analyze image to determine optimal enhancement parameters
	analysis := ip.analyzeImage(img)

	config := EnhancementConfig{
		Sharpen:    analysis.NeedsSharpening,
		Denoise:    analysis.NeedsDenoising,
		Contrast:   analysis.ContrastAdjustment,
		Brightness: analysis.BrightnessAdjustment,
		Saturation: analysis.SaturationAdjustment,
		Gamma:      analysis.GammaCorrection,
	}

	logger.Info("Applying auto enhancement: brightness=%.2f, contrast=%.2f, saturation=%.2f",
		config.Brightness, config.Contrast, config.Saturation)

	return ip.EnhanceImage(img, config)
}

// ImageAnalysis holds the results of image analysis
type ImageAnalysis struct {
	NeedsSharpening       bool
	NeedsDenoising        bool
	ContrastAdjustment    float64
	BrightnessAdjustment  float64
	SaturationAdjustment  float64
	GammaCorrection       float64
	AverageBrightness     float64
	Contrast              float64
	Saturation            float64
}

// analyzeImage analyzes image characteristics to determine enhancement needs
func (ip *ImageProcessor) analyzeImage(img image.Image) ImageAnalysis {
	bounds := img.Bounds()
	pixelCount := (bounds.Max.X - bounds.Min.X) * (bounds.Max.Y - bounds.Min.Y)

	var totalR, totalG, totalB float64
	var minBrightness, maxBrightness float64 = 255, 0
	var totalSaturation float64

	// First pass: calculate averages and ranges
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			pixel := img.At(x, y)
			r, g, b, _ := pixel.RGBA()

			r8 := float64(r >> 8)
			g8 := float64(g >> 8)
			b8 := float64(b >> 8)

			totalR += r8
			totalG += g8
			totalB += b8

			// Calculate brightness (luminance)
			brightness := 0.299*r8 + 0.587*g8 + 0.114*b8
			if brightness < minBrightness {
				minBrightness = brightness
			}
			if brightness > maxBrightness {
				maxBrightness = brightness
			}

			// Calculate saturation
			_, s, _ := ip.rgbToHsv(r8, g8, b8)
			totalSaturation += s
		}
	}

	avgR := totalR / float64(pixelCount)
	avgG := totalG / float64(pixelCount)
	avgB := totalB / float64(pixelCount)
	avgBrightness := 0.299*avgR + 0.587*avgG + 0.114*avgB
	avgSaturation := totalSaturation / float64(pixelCount)
	contrast := maxBrightness - minBrightness

	// Determine enhancement needs
	analysis := ImageAnalysis{
		AverageBrightness: avgBrightness,
		Contrast:          contrast,
		Saturation:        avgSaturation,
	}

	// Brightness adjustment
	if avgBrightness < 100 {
		analysis.BrightnessAdjustment = 0.2 // Brighten dark images
	} else if avgBrightness > 180 {
		analysis.BrightnessAdjustment = -0.1 // Darken bright images
	}

	// Contrast adjustment
	if contrast < 100 {
		analysis.ContrastAdjustment = 0.3 // Increase contrast for flat images
	} else if contrast > 200 {
		analysis.ContrastAdjustment = -0.1 // Reduce contrast for high-contrast images
	}

	// Saturation adjustment
	if avgSaturation < 0.3 {
		analysis.SaturationAdjustment = 0.2 // Increase saturation for dull images
	} else if avgSaturation > 0.8 {
		analysis.SaturationAdjustment = -0.1 // Reduce saturation for oversaturated images
	}

	// Gamma correction
	if avgBrightness < 80 {
		analysis.GammaCorrection = 0.8 // Lighten shadows
	} else if avgBrightness > 200 {
		analysis.GammaCorrection = 1.2 // Darken highlights
	} else {
		analysis.GammaCorrection = 1.0
	}

	// Sharpening (based on contrast)
	analysis.NeedsSharpening = contrast < 80

	// Denoising (conservative approach)
	analysis.NeedsDenoising = false // Can be enabled based on specific criteria

	return analysis
}