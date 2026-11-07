// 账单凭证截图上传：与房间照片同走「服务端中转 → 七牛/本地对象存储」链路。
package handlers

import (
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"rental-server/logger"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// billEvidenceExts 账单凭证允许的图片扩展名：常见截图/照片格式尽量全收。
// svg 可内嵌脚本有 XSS 风险，不入列；avif/heic/heif/tiff 浏览器端兼容性差异较大，
// 上传允许、前端对无法预览的格式降级为「下载查看」。
var billEvidenceExts = map[string]bool{
	".jpg": true, ".jpeg": true, ".png": true, ".gif": true, ".webp": true,
	".bmp": true, ".tif": true, ".tiff": true, ".avif": true, ".heic": true, ".heif": true,
}

// evidenceSniffBlindExts http.DetectContentType 识别不了的图片格式（按扩展名放行内容校验）。
var evidenceSniffBlindExts = map[string]bool{
	".avif": true, ".heic": true, ".heif": true, ".tif": true, ".tiff": true,
}

// billEvidenceMaxSize 凭证图片大小上限（与房间照片一致）。
const billEvidenceMaxSize = 10 * 1024 * 1024

// UploadBillEvidence 上传账单凭证截图（仅图片、多格式）。
// 只负责把文件放入对象存储并返回 key；是否挂账由账单创建/补充接口决定，
// 上传后未随账单提交的孤儿文件由定时 GC 在宽限期后清理。
func (h *MediaHandler) UploadBillEvidence(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	building, err := h.MediaService.GetBuildingByID(bid)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	if !billEvidenceExts[ext] {
		utils.Error(c, http.StatusBadRequest, "仅支持图片格式：jpg/png/gif/webp/bmp/tiff/avif/heic/heif")
		return
	}
	if header.Size > billEvidenceMaxSize {
		utils.Error(c, http.StatusBadRequest, "图片不能超过10MB")
		return
	}
	// 内容嗅探兜底：扩展名可伪造，真实内容须为图片；
	// avif/heic/heif/tiff 的魔数 DetectContentType 不认识，按扩展名放行
	buf := make([]byte, 512)
	n, _ := io.ReadFull(file, buf)
	file.Seek(0, io.SeekStart)
	if n > 0 && !evidenceSniffBlindExts[ext] {
		if mimeType := http.DetectContentType(buf[:n]); !strings.HasPrefix(mimeType, "image/") {
			utils.Error(c, http.StatusBadRequest, "文件内容不是图片")
			return
		}
	}

	uuidName := uuid.New().String() + ext
	nameSlug := utils.Slugify(building.Name)
	key := filepath.ToSlash(filepath.Join("buildings", fmt.Sprintf("%d_%s", bid, nameSlug), "bills", uuidName))
	if err := h.uploadToStorage(key, file, header.Size); err != nil {
		logger.Log.Error().Err(err).Str("key", key).Msg("账单凭证上传失败")
		utils.Error(c, http.StatusInternalServerError, "上传凭证失败")
		return
	}
	logger.Log.Info().Uint("building_id", bid).Str("key", key).Msg("账单凭证上传成功")
	utils.Created(c, "上传成功", gin.H{
		"file_key":  key,
		"file_name": header.Filename,
		"file_size": header.Size,
	})
}
