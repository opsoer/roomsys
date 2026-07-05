package utils

import (
	"bytes"
	"image"
	"image/jpeg"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"strings"

	"rental-server/logger"

	"golang.org/x/image/draw"
)

const (
	MaxImageWidth  = 1920
	MaxImageHeight = 1920
	ThumbWidth     = 300
	JPEGQuality    = 80
)

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

func ProcessImageNoThumb(data []byte) ([]byte, error) {
	img, format, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	resized := resizeImage(img, MaxImageWidth, MaxImageHeight)
	return encodeImage(resized, format)
}

func ProcessImageToFile(input io.Reader, savePath, thumbPath string) error {
	img, format, err := image.Decode(input)
	if err != nil {
		return err
	}

	resized := resizeImage(img, MaxImageWidth, MaxImageHeight)
	if err := saveImageToFile(resized, savePath, format); err != nil {
		return err
	}

	if thumbPath != "" {
		thumb := resizeImage(img, ThumbWidth, 0)
		if err := saveImageToFile(thumb, thumbPath, "jpeg"); err != nil {
			logger.Log.Warn().Err(err).Msg("生成缩略图失败")
		}
	}
	return nil
}

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

func saveImageToFile(img image.Image, path, format string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()

	format = strings.ToLower(format)
	switch format {
	case "jpeg", "jpg":
		return jpeg.Encode(f, img, &jpeg.Options{Quality: JPEGQuality})
	case "png":
		return png.Encode(f, img)
	default:
		return jpeg.Encode(f, img, &jpeg.Options{Quality: JPEGQuality})
	}
}

func IsImageType(mimeType string) bool {
	return strings.HasPrefix(mimeType, "image/")
}
