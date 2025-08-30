package watermark

import (
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
)

// WatermarkStyle 表示不同的水印样式
type WatermarkStyle int

const (
	StyleNormal WatermarkStyle = iota
	StyleTransparent
	StyleShadow
	StyleOutline
	StyleGradient
)

// WatermarkPosition 表示高级定位选项
type WatermarkPosition int

const (
	PosTopLeft WatermarkPosition = iota
	PosTopRight
	PosBottomLeft
	PosBottomRight
	PosCenter
	PosTopCenter
	PosBottomCenter
	PosCenterLeft
	PosCenterRight
)

// AdvancedWatermarkConfig 保存高级水印处理的配置
type AdvancedWatermarkConfig struct {
	Style       WatermarkStyle
	Position    WatermarkPosition
	Opacity     float64 // 0.0 到 1.0
	Scale       float64 // 水印大小的缩放因子
	Rotation    float64 // 旋转角度（度）
	MarginX     int     // 距离边缘的水平边距
	MarginY     int     // 距离边缘的垂直边距
	ShadowColor color.RGBA
	OutlineColor color.RGBA
}

// DefaultAdvancedConfig 返回高级水印的默认配置
func DefaultAdvancedConfig() AdvancedWatermarkConfig {
	return AdvancedWatermarkConfig{
		Style:        StyleNormal,
		Position:     PosBottomRight,
		Opacity:      1.0,
		Scale:        1.0,
		Rotation:     0.0,
		MarginX:      10,
		MarginY:      10,
		ShadowColor:  color.RGBA{0, 0, 0, 128},
		OutlineColor: color.RGBA{255, 255, 255, 255},
	}
}

// AddAdvancedWatermark 添加具有高级样式选项的水印
func (wp *WatermarkProcessor) AddAdvancedWatermark(baseImg *image.RGBA, watermarkImg image.Image, config AdvancedWatermarkConfig) error {
	if watermarkImg == nil {
		return fmt.Errorf("watermark image is nil")
	}

	// 计算缩放后的水印大小
	wmBounds := watermarkImg.Bounds()
	scaledWidth := int(float64(wmBounds.Dx()) * config.Scale)
	scaledHeight := int(float64(wmBounds.Dy()) * config.Scale)

	// 如果需要，调整水印大小
	if config.Scale != 1.0 {
		watermarkImg = wp.resizeImageAdvanced(watermarkImg, scaledWidth, scaledHeight)
	}

	// 如果需要，应用旋转
	if config.Rotation != 0.0 {
		watermarkImg = wp.rotateImage(watermarkImg, config.Rotation)
		// 旋转后更新边界
		wmBounds = watermarkImg.Bounds()
		scaledWidth = wmBounds.Dx()
		scaledHeight = wmBounds.Dy()
	}

	// 计算位置
	baseBounds := baseImg.Bounds()
	pos := wp.calculateAdvancedPosition(baseBounds, scaledWidth, scaledHeight, config)

	// 根据样式应用水印
	switch config.Style {
	case StyleNormal:
		wp.applyNormalWatermark(baseImg, watermarkImg, pos, config.Opacity)
	case StyleTransparent:
		wp.applyTransparentWatermark(baseImg, watermarkImg, pos, config.Opacity)
	case StyleShadow:
		wp.applyShadowWatermark(baseImg, watermarkImg, pos, config)
	case StyleOutline:
		wp.applyOutlineWatermark(baseImg, watermarkImg, pos, config)
	case StyleGradient:
		wp.applyGradientWatermark(baseImg, watermarkImg, pos, config)
	default:
		wp.applyNormalWatermark(baseImg, watermarkImg, pos, config.Opacity)
	}

	return nil
}

// calculateAdvancedPosition 基于高级定位计算水印位置
func (wp *WatermarkProcessor) calculateAdvancedPosition(baseBounds image.Rectangle, wmWidth, wmHeight int, config AdvancedWatermarkConfig) Position {
	baseWidth := baseBounds.Dx()
	baseHeight := baseBounds.Dy()

	var x, y int

	switch config.Position {
	case PosTopLeft:
		x = config.MarginX
		y = config.MarginY
	case PosTopRight:
		x = baseWidth - wmWidth - config.MarginX
		y = config.MarginY
	case PosBottomLeft:
		x = config.MarginX
		y = baseHeight - wmHeight - config.MarginY
	case PosBottomRight:
		x = baseWidth - wmWidth - config.MarginX
		y = baseHeight - wmHeight - config.MarginY
	case PosCenter:
		x = (baseWidth - wmWidth) / 2
		y = (baseHeight - wmHeight) / 2
	case PosTopCenter:
		x = (baseWidth - wmWidth) / 2
		y = config.MarginY
	case PosBottomCenter:
		x = (baseWidth - wmWidth) / 2
		y = baseHeight - wmHeight - config.MarginY
	case PosCenterLeft:
		x = config.MarginX
		y = (baseHeight - wmHeight) / 2
	case PosCenterRight:
		x = baseWidth - wmWidth - config.MarginX
		y = (baseHeight - wmHeight) / 2
	default:
		x = baseWidth - wmWidth - config.MarginX
		y = baseHeight - wmHeight - config.MarginY
	}

	// 确保位置在边界内
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x+wmWidth > baseWidth {
		x = baseWidth - wmWidth
	}
	if y+wmHeight > baseHeight {
		y = baseHeight - wmHeight
	}

	return Position{X: x, Y: y}
}

// applyNormalWatermark 应用普通水印
func (wp *WatermarkProcessor) applyNormalWatermark(baseImg *image.RGBA, watermarkImg image.Image, pos Position, opacity float64) {
	wmBounds := watermarkImg.Bounds()
	targetRect := image.Rect(pos.X, pos.Y, pos.X+wmBounds.Dx(), pos.Y+wmBounds.Dy())

	if opacity >= 1.0 {
		// 完全不透明 - 直接绘制
		draw.Draw(baseImg, targetRect, watermarkImg, wmBounds.Min, draw.Over)
	} else {
		// 应用透明度
		wp.drawWithOpacity(baseImg, watermarkImg, targetRect, opacity)
	}
}

// applyTransparentWatermark 应用透明水印
func (wp *WatermarkProcessor) applyTransparentWatermark(baseImg *image.RGBA, watermarkImg image.Image, pos Position, opacity float64) {
	// 透明水印就是降低透明度的普通水印
	wp.applyNormalWatermark(baseImg, watermarkImg, pos, opacity*0.5)
}

// applyShadowWatermark 应用带阴影效果的水印
func (wp *WatermarkProcessor) applyShadowWatermark(baseImg *image.RGBA, watermarkImg image.Image, pos Position, config AdvancedWatermarkConfig) {
	// 首先绘制阴影（偏移几个像素）
	shadowPos := Position{X: pos.X + 3, Y: pos.Y + 3}
	wp.drawShadow(baseImg, watermarkImg, shadowPos, config.ShadowColor)

	// 然后绘制实际的水印
	wp.applyNormalWatermark(baseImg, watermarkImg, pos, config.Opacity)
}

// applyOutlineWatermark 应用带轮廓效果的水印
func (wp *WatermarkProcessor) applyOutlineWatermark(baseImg *image.RGBA, watermarkImg image.Image, pos Position, config AdvancedWatermarkConfig) {
	// 在多个方向绘制轮廓
	offsets := []Position{
		{X: -1, Y: -1}, {X: 0, Y: -1}, {X: 1, Y: -1},
		{X: -1, Y: 0}, {X: 1, Y: 0},
		{X: -1, Y: 1}, {X: 0, Y: 1}, {X: 1, Y: 1},
	}

	for _, offset := range offsets {
		outlinePos := Position{X: pos.X + offset.X, Y: pos.Y + offset.Y}
		wp.drawOutline(baseImg, watermarkImg, outlinePos, config.OutlineColor)
	}

	// 绘制实际的水印
	wp.applyNormalWatermark(baseImg, watermarkImg, pos, config.Opacity)
}

// applyGradientWatermark 应用带渐变效果的水印
func (wp *WatermarkProcessor) applyGradientWatermark(baseImg *image.RGBA, watermarkImg image.Image, pos Position, config AdvancedWatermarkConfig) {
	// 创建渐变蒙版并应用水印
	wmBounds := watermarkImg.Bounds()
	for y := 0; y < wmBounds.Dy(); y++ {
		for x := 0; x < wmBounds.Dx(); x++ {
			// 基于位置计算渐变透明度
			gradientOpacity := float64(y) / float64(wmBounds.Dy())
			finalOpacity := config.Opacity * gradientOpacity

			// 获取水印像素
			wmColor := watermarkImg.At(wmBounds.Min.X+x, wmBounds.Min.Y+y)
			r, g, b, a := wmColor.RGBA()

			// 应用渐变透明度
			newA := uint8(float64(a>>8) * finalOpacity)
			newColor := color.RGBA{uint8(r >> 8), uint8(g >> 8), uint8(b >> 8), newA}

			// 在基础图像上设置像素
			baseImg.Set(pos.X+x, pos.Y+y, newColor)
		}
	}
}

// drawWithOpacity 以指定透明度绘制水印
func (wp *WatermarkProcessor) drawWithOpacity(baseImg *image.RGBA, watermarkImg image.Image, targetRect image.Rectangle, opacity float64) {
	wmBounds := watermarkImg.Bounds()
	for y := targetRect.Min.Y; y < targetRect.Max.Y; y++ {
		for x := targetRect.Min.X; x < targetRect.Max.X; x++ {
			wmX := wmBounds.Min.X + (x - targetRect.Min.X)
			wmY := wmBounds.Min.Y + (y - targetRect.Min.Y)

			// 获取颜色
			baseColor := baseImg.At(x, y)
			wmColor := watermarkImg.At(wmX, wmY)

			// 以透明度混合颜色
			blendedColor := wp.blendColors(baseColor, wmColor, opacity)
			baseImg.Set(x, y, blendedColor)
		}
	}
}

// drawShadow 绘制阴影效果
func (wp *WatermarkProcessor) drawShadow(baseImg *image.RGBA, watermarkImg image.Image, pos Position, shadowColor color.RGBA) {
	wmBounds := watermarkImg.Bounds()
	for y := 0; y < wmBounds.Dy(); y++ {
		for x := 0; x < wmBounds.Dx(); x++ {
			// 检查水印像素是否不透明
			_, _, _, a := watermarkImg.At(wmBounds.Min.X+x, wmBounds.Min.Y+y).RGBA()
			if a > 0 {
				// 绘制阴影像素
				baseImg.Set(pos.X+x, pos.Y+y, shadowColor)
			}
		}
	}
}

// drawOutline 绘制轮廓效果
func (wp *WatermarkProcessor) drawOutline(baseImg *image.RGBA, watermarkImg image.Image, pos Position, outlineColor color.RGBA) {
	wmBounds := watermarkImg.Bounds()
	for y := 0; y < wmBounds.Dy(); y++ {
		for x := 0; x < wmBounds.Dx(); x++ {
			// 检查水印像素是否不透明
			_, _, _, a := watermarkImg.At(wmBounds.Min.X+x, wmBounds.Min.Y+y).RGBA()
			if a > 0 {
				// 绘制轮廓像素
				baseImg.Set(pos.X+x, pos.Y+y, outlineColor)
			}
		}
	}
}

// blendColors 以指定透明度混合两种颜色
func (wp *WatermarkProcessor) blendColors(base, overlay color.Color, opacity float64) color.RGBA {
	br, bg, bb, ba := base.RGBA()
	or, og, ob, oa := overlay.RGBA()

	// 转换为8位
	br8, bg8, bb8, ba8 := uint8(br>>8), uint8(bg>>8), uint8(bb>>8), uint8(ba>>8)
	or8, og8, ob8, oa8 := uint8(or>>8), uint8(og>>8), uint8(ob>>8), uint8(oa>>8)

	// 对覆盖层透明度应用透明度
	oa8 = uint8(float64(oa8) * opacity)

	// Alpha混合
	alpha := float64(oa8) / 255.0
	invAlpha := 1.0 - alpha

	r := uint8(float64(or8)*alpha + float64(br8)*invAlpha)
	g := uint8(float64(og8)*alpha + float64(bg8)*invAlpha)
	b := uint8(float64(ob8)*alpha + float64(bb8)*invAlpha)
	a := uint8(math.Max(float64(ba8), float64(oa8)))

	return color.RGBA{r, g, b, a}
}

// resizeImageAdvanced 以更好的质量调整图像大小（双线性插值）
func (wp *WatermarkProcessor) resizeImageAdvanced(src image.Image, width, height int) image.Image {
	srcBounds := src.Bounds()
	dst := image.NewRGBA(image.Rect(0, 0, width, height))

	xRatio := float64(srcBounds.Dx()) / float64(width)
	yRatio := float64(srcBounds.Dy()) / float64(height)

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 双线性插值
			srcX := float64(x) * xRatio
			srcY := float64(y) * yRatio

			x1 := int(srcX)
			y1 := int(srcY)
			x2 := x1 + 1
			y2 := y1 + 1

			// 确保边界
			if x2 >= srcBounds.Dx() {
				x2 = srcBounds.Dx() - 1
			}
			if y2 >= srcBounds.Dy() {
				y2 = srcBounds.Dy() - 1
			}

			// 获取周围像素
			c1 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y1)
			c2 := src.At(srcBounds.Min.X+x2, srcBounds.Min.Y+y1)
			c3 := src.At(srcBounds.Min.X+x1, srcBounds.Min.Y+y2)
			c4 := src.At(srcBounds.Min.X+x2, srcBounds.Min.Y+y2)

			// 插值
			interpolated := wp.bilinearInterpolate(c1, c2, c3, c4, srcX-float64(x1), srcY-float64(y1))
			dst.Set(x, y, interpolated)
		}
	}

	return dst
}

// bilinearInterpolate 对四种颜色执行双线性插值
func (wp *WatermarkProcessor) bilinearInterpolate(c1, c2, c3, c4 color.Color, fx, fy float64) color.RGBA {
	r1, g1, b1, a1 := c1.RGBA()
	r2, g2, b2, a2 := c2.RGBA()
	r3, g3, b3, a3 := c3.RGBA()
	r4, g4, b4, a4 := c4.RGBA()

	// 转换为float64进行插值
	f1r, f1g, f1b, f1a := float64(r1), float64(g1), float64(b1), float64(a1)
	f2r, f2g, f2b, f2a := float64(r2), float64(g2), float64(b2), float64(a2)
	f3r, f3g, f3b, f3a := float64(r3), float64(g3), float64(b3), float64(a3)
	f4r, f4g, f4b, f4a := float64(r4), float64(g4), float64(b4), float64(a4)

	// 双线性插值
	top_r := f1r*(1-fx) + f2r*fx
	top_g := f1g*(1-fx) + f2g*fx
	top_b := f1b*(1-fx) + f2b*fx
	top_a := f1a*(1-fx) + f2a*fx

	bot_r := f3r*(1-fx) + f4r*fx
	bot_g := f3g*(1-fx) + f4g*fx
	bot_b := f3b*(1-fx) + f4b*fx
	bot_a := f3a*(1-fx) + f4a*fx

	final_r := top_r*(1-fy) + bot_r*fy
	final_g := top_g*(1-fy) + bot_g*fy
	final_b := top_b*(1-fy) + bot_b*fy
	final_a := top_a*(1-fy) + bot_a*fy

	return color.RGBA{
		uint8(int(final_r) >> 8),
		uint8(int(final_g) >> 8),
		uint8(int(final_b) >> 8),
		uint8(int(final_a) >> 8),
	}
}

// rotateImage 按指定角度（度）旋转图像
func (wp *WatermarkProcessor) rotateImage(src image.Image, angle float64) image.Image {
	if angle == 0 {
		return src
	}

	// 将角度转换为弧度
	radians := angle * math.Pi / 180.0
	cos := math.Cos(radians)
	sin := math.Sin(radians)

	srcBounds := src.Bounds()
	srcWidth := float64(srcBounds.Dx())
	srcHeight := float64(srcBounds.Dy())

	// 计算旋转后的新尺寸
	newWidth := int(math.Abs(srcWidth*cos) + math.Abs(srcHeight*sin))
	newHeight := int(math.Abs(srcWidth*sin) + math.Abs(srcHeight*cos))

	dst := image.NewRGBA(image.Rect(0, 0, newWidth, newHeight))

	// 中心点
	centerX := float64(newWidth) / 2
	centerY := float64(newHeight) / 2
	srcCenterX := srcWidth / 2
	srcCenterY := srcHeight / 2

	// 旋转每个像素
	for y := 0; y < newHeight; y++ {
		for x := 0; x < newWidth; x++ {
			// 平移到中心
			tx := float64(x) - centerX
			ty := float64(y) - centerY

			// 旋转（逆变换）
			srcX := tx*cos + ty*sin + srcCenterX
			srcY := -tx*sin + ty*cos + srcCenterY

			// 检查边界
			if srcX >= 0 && srcX < srcWidth && srcY >= 0 && srcY < srcHeight {
				// 从源获取像素
				pixel := src.At(srcBounds.Min.X+int(srcX), srcBounds.Min.Y+int(srcY))
				dst.Set(x, y, pixel)
			}
		}
	}

	return dst
}