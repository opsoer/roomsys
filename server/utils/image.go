// 工具包，提供图片处理功能（缩放、编码、缩略图生成）
package utils

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"strings"

	"rental-server/logger"

	"golang.org/x/image/draw"
)

// 图片处理常量配置
const (
	MaxImageWidth  = 1600 // 最大图片宽度
	MaxImageHeight = 1600 // 最大图片高度
	ThumbWidth     = 300  // 缩略图宽度
	JPEGQuality    = 65   // JPEG 压缩质量
)

// ProcessImageBytes 处理图片字节数据，返回处理后的图片和缩略图
func ProcessImageBytes(data []byte, ext string) (processed, thumbnail []byte, err error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}

	resized := resizeImage(img, MaxImageWidth, MaxImageHeight)
	processed, err = encodeImage(resized, format)
	if err != nil {
		return nil, nil, err
	}

	thumb := resizeImage(img, ThumbWidth, 0)
	thumbnail, err = encodeImage(thumb, "jpeg")
	if err != nil {
		logger.Log.Warn().Err(err).Msg("生成缩略图失败")
	}

	return processed, thumbnail, nil
}

// ProcessImageNoThumb 处理图片字节数据，不生成缩略图
func ProcessImageNoThumb(data []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	resized := resizeImage(img, MaxImageWidth, MaxImageHeight)
	return encodeImage(resized, format)
}

// resizeImage 按比例缩放图片至指定最大尺寸
func resizeImage(img image.Image, maxW, maxH int) image.Image {
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()

	if maxH == 0 {
		maxH = h
	}

	if w <= maxW && h <= maxH {
		return img
	}

	ratioW := float64(w) / float64(maxW)
	ratioH := float64(h) / float64(maxH)
	ratio := ratioW
	if ratioH > ratio {
		ratio = ratioH
	}

	newW := int(float64(w) / ratio)
	newH := int(float64(h) / ratio)

	dst := image.NewRGBA(image.Rect(0, 0, newW, newH))
	draw.ApproxBiLinear.Scale(dst, dst.Bounds(), img, img.Bounds(), draw.Over, nil)
	return dst
}

// encodeImage 将图片按指定格式编码为字节数据
func encodeImage(img image.Image, format string) ([]byte, error) {
	var buf bytes.Buffer
	format = strings.ToLower(format)
	switch format {
	case "jpeg", "jpg":
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: JPEGQuality})
		return buf.Bytes(), err
	case "png":
		err := png.Encode(&buf, img)
		return buf.Bytes(), err
	default:
		err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: JPEGQuality})
		return buf.Bytes(), err
	}
}

