package watermark

import (
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"image/png"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"movie-data-capture/internal/config"
	"movie-data-capture/pkg/logger"
)

// WatermarkType 表示不同类型的水印（基于Python版本增强）
type WatermarkType int

const (
	Subtitle WatermarkType = iota + 1
	Leak
	Uncensored
	FourK
	EightK
	ISO
	Youma    // 有码
	UMR      // 破解
)

// Position 表示水印在图像上的位置
type Position struct {
	X, Y int
}

// WatermarkProcessor 处理水印操作
type WatermarkProcessor struct {
	config *config.Config
}

// NewWatermarkProcessor 创建一个新的水印处理器
func NewWatermarkProcessor(cfg *config.Config) *WatermarkProcessor {
	return &WatermarkProcessor{
		config: cfg,
	}
}

// AddWatermarks 向海报和缩略图添加水印（旧版接口）
func (wp *WatermarkProcessor) AddWatermarks(posterPath, thumbPath string, cnSub, leak, uncensored, hack, fourK, iso bool) error {
	return wp.AddWatermarksExtended(posterPath, thumbPath, cnSub, leak, uncensored, hack, fourK, false, iso, false, hack)
}

// AddWatermarksExtended 使用扩展选项添加水印（基于Python版本）
func (wp *WatermarkProcessor) AddWatermarksExtended(posterPath, thumbPath string, cnSub, leak, uncensored, hack, fourK, eightK, iso, youma, umr bool) error {
	if !wp.config.Watermark.Switch {
		return nil
	}

	// 构建水印类型描述
	var markTypes []string
	if cnSub {
		markTypes = append(markTypes, "字幕")
	}
	if leak {
		markTypes = append(markTypes, "流出")
	}
	if uncensored {
		markTypes = append(markTypes, "无码")
	}
	if umr {
		markTypes = append(markTypes, "破解")
	}
	if fourK {
		markTypes = append(markTypes, "4K")
	}
	if eightK {
		markTypes = append(markTypes, "8K")
	}
	if iso {
		markTypes = append(markTypes, "ISO")
	}
	if youma {
		markTypes = append(markTypes, "有码")
	}
	if umr {
		markTypes = append(markTypes, "无码流出")
	}

	if len(markTypes) == 0 {
		return nil
	}

	// 向海报和缩略图添加水印
	err := wp.addWatermarksToImageExtended(posterPath, cnSub, leak, uncensored, hack, fourK, eightK, iso, youma, umr)
	if err != nil {
		logger.Warn("Failed to add watermarks to poster: %v", err)
	}

	err = wp.addWatermarksToImageExtended(thumbPath, cnSub, leak, uncensored, hack, fourK, eightK, iso, youma, umr)
	if err != nil {
		logger.Warn("Failed to add watermarks to thumbnail: %v", err)
	}

	logger.Info("[+]Add Mark: %s", strings.Join(markTypes, ","))
	return nil
}

// addWatermarksToImage 向单个图像添加水印（旧版接口）
func (wp *WatermarkProcessor) addWatermarksToImage(imagePath string, cnSub, leak, uncensored, hack, fourK, iso bool) error {
	return wp.addWatermarksToImageExtended(imagePath, cnSub, leak, uncensored, hack, fourK, false, iso, false, hack)
}

// addWatermarksToImageExtended 使用扩展类型向单个图像添加水印
func (wp *WatermarkProcessor) addWatermarksToImageExtended(imagePath string, cnSub, leak, uncensored, hack, fourK, eightK, iso, youma, umr bool) error {
	if imagePath == "" {
		return nil
	}

	// 打开基础图像
	baseImg, err := wp.openImage(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image %s: %w", imagePath, err)
	}

	// 创建一个新的RGBA图像用于绘制
	bounds := baseImg.Bounds()
	rgbaImg := image.NewRGBA(bounds)
	draw.Draw(rgbaImg, bounds, baseImg, bounds.Min, draw.Src)

	// 获取水印位置计数器（起始位置）
	count := wp.config.Watermark.Water % 4
	size := 9 // 水印缩放的尺寸除数

	// 按顺序添加水印
	if cnSub && !leak && !umr {
		err = wp.addSingleWatermark(rgbaImg, size, count, Subtitle)
		if err != nil {
			logger.Warn("Failed to add subtitle watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if leak {
		err = wp.addSingleWatermark(rgbaImg, size, count, Leak)
		if err != nil {
			logger.Warn("Failed to add leak watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if uncensored {
		err = wp.addSingleWatermark(rgbaImg, size, count, Uncensored)
		if err != nil {
			logger.Warn("Failed to add uncensored watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if umr {
		err = wp.addSingleWatermark(rgbaImg, size, count, UMR)
		if err != nil {
			logger.Warn("Failed to add UMR watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if fourK {
		err = wp.addSingleWatermark(rgbaImg, size, count, FourK)
		if err != nil {
			logger.Warn("Failed to add 4K watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if eightK {
		err = wp.addSingleWatermark(rgbaImg, size, count, EightK)
		if err != nil {
			logger.Warn("Failed to add 8K watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if iso {
		err = wp.addSingleWatermark(rgbaImg, size, count, ISO)
		if err != nil {
			logger.Warn("Failed to add ISO watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if youma {
		err = wp.addSingleWatermark(rgbaImg, size, count, Youma)
		if err != nil {
			logger.Warn("Failed to add youma watermark: %v", err)
		}
		count = (count + 1) % 4
	}

	if umr {
		err = wp.addSingleWatermark(rgbaImg, size, count, UMR)
		if err != nil {
			logger.Warn("Failed to add UMR watermark: %v", err)
		}
	}

	// 保存修改后的图像
	return wp.saveImage(rgbaImg, imagePath)
}

// addSingleWatermark 向图像添加单个水印
func (wp *WatermarkProcessor) addSingleWatermark(baseImg *image.RGBA, size, position int, wmType WatermarkType) error {
	// 获取水印图像
	watermarkImg, err := wp.getWatermarkImage(wmType)
	if err != nil {
		return fmt.Errorf("failed to get watermark image: %w", err)
	}

	// 计算水印大小和位置
	bounds := baseImg.Bounds()
	height := bounds.Dy() / size
	width := height * watermarkImg.Bounds().Dx() / watermarkImg.Bounds().Dy()

	// 调整水印图像大小
	resizedWatermark := wp.resizeImage(watermarkImg, width, height)

	// 计算位置（0: 左上角, 1: 右上角, 2: 右下角, 3: 左下角）
	var pos Position
	switch position {
	case 0: // 左上角
		pos = Position{X: 0, Y: 0}
	case 1: // 右上角
		pos = Position{X: bounds.Dx() - width, Y: 0}
	case 2: // 右下角
		pos = Position{X: bounds.Dx() - width, Y: bounds.Dy() - height}
	case 3: // 左下角
		pos = Position{X: 0, Y: bounds.Dy() - height}
	}

	// 在基础图像上绘制水印
	watermarkBounds := image.Rect(pos.X, pos.Y, pos.X+width, pos.Y+height)
	draw.Draw(baseImg, watermarkBounds, resizedWatermark, image.Point{}, draw.Over)

	return nil
}

// getWatermarkImage 加载指定类型的水印图像
func (wp *WatermarkProcessor) getWatermarkImage(wmType WatermarkType) (image.Image, error) {
	var filename string
	switch wmType {
	case Subtitle:
		filename = "SUB.png"
	case Leak:
		filename = "LEAK.png"
	case Uncensored:
		filename = "UNCENSORED.png"
	case FourK:
		filename = "4K.png"
	case EightK:
		filename = "8K.png"
	case ISO:
		filename = "ISO.png"
	case Youma:
		filename = "YOUMA.png"
	case UMR:
		filename = "UMR.png"
	default:
		return nil, fmt.Errorf("invalid watermark type: %d", wmType)
	}

	// 首先尝试加载本地图像
	localPath := filepath.Join("Img", filename)
	if _, err := os.Stat(localPath); err == nil {
		return wp.openImage(localPath)
	}

	// 如果本地图像不存在，则从GitHub下载
	url := fmt.Sprintf("https://raw.githubusercontent.com/yoshiko2/AV_Data_Capture/master/Img/%s", filename)
	return wp.downloadWatermarkImage(url)
}

// downloadWatermarkImage 从URL下载水印图像
func (wp *WatermarkProcessor) downloadWatermarkImage(url string) (image.Image, error) {
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to download watermark image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to download watermark image: status %d", resp.StatusCode)
	}

	img, _, err := image.Decode(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to decode watermark image: %w", err)
	}

	return img, nil
}

// openImage 打开图像文件
func (wp *WatermarkProcessor) openImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

// saveImage 将图像保存到文件
func (wp *WatermarkProcessor) saveImage(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	// 根据文件扩展名确定格式
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".jpg", ".jpeg":
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
	case ".png":
		return png.Encode(file, img)
	default:
		// 默认为JPEG
		return jpeg.Encode(file, img, &jpeg.Options{Quality: 95})
	}
}

// resizeImage 使用最近邻算法将图像调整为指定尺寸
func (wp *WatermarkProcessor) resizeImage(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	// 简单的最近邻缩放
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			srcX := x * srcBounds.Dx() / width
			srcY := y * srcBounds.Dy() / height
			dst.Set(x, y, src.At(srcBounds.Min.X+srcX, srcBounds.Min.Y+srcY))
		}
	}

	return dst
}