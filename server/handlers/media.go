package handlers

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	qiniuAuth "github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"gorm.io/gorm"
)

type MediaHandler struct {
	DB  *gorm.DB
	Cfg *config.Config
}

var allowedMIMEs = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
}

var maxSizes = map[string]int64{
	"image": 10 * 1024 * 1024,
	"video": 200 * 1024 * 1024,
}

func getZone(name string) *storage.Region {
	switch name {
	case "z0":
		return &storage.ZoneHuadong
	case "z1":
		return &storage.ZoneHuabei
	case "z2":
		return &storage.ZoneHuanan
	case "na0":
		return &storage.ZoneBeimei
	default:
		return &storage.ZoneHuadong
	}
}

var (
	qiniuDomain   string
	qiniuDomainMu sync.RWMutex
)

func (h *MediaHandler) useQiniu() bool {
	return h.Cfg.QiniuAccessKey != "" && h.Cfg.QiniuBucket != ""
}

func (h *MediaHandler) qiniuMac() *qiniuAuth.Mac {
	return qiniuAuth.NewMac(h.Cfg.QiniuAccessKey, h.Cfg.QiniuSecretKey)
}

func (h *MediaHandler) qiniuConfig() storage.Config {
	return storage.Config{Region: getZone(h.Cfg.QiniuZone), UseHTTPS: h.Cfg.QiniuUseHTTPS}
}

func (h *MediaHandler) qiniuUpload(key string, reader io.Reader, size int64) error {
	putPolicy := storage.PutPolicy{
		Scope:   h.Cfg.QiniuBucket,
		Expires: 3600,
	}
	upToken := putPolicy.UploadToken(h.qiniuMac())
	cfg := h.qiniuConfig()
	formUploader := storage.NewFormUploader(&cfg)
	ret := storage.PutRet{}
	return formUploader.Put(context.Background(), &ret, upToken, key, reader, size, nil)
}

func (h *MediaHandler) qiniuDelete(key string) error {
	cfg := h.qiniuConfig()
	bucketMgr := storage.NewBucketManager(h.qiniuMac(), &cfg)
	return bucketMgr.Delete(h.Cfg.QiniuBucket, key)
}

func (h *MediaHandler) getDomain() (string, error) {
	qiniuDomainMu.RLock()
	d := qiniuDomain
	qiniuDomainMu.RUnlock()
	if d != "" {
		return d, nil
	}

	cfg := h.qiniuConfig()
	bucketMgr := storage.NewBucketManager(h.qiniuMac(), &cfg)
	domains, err := bucketMgr.ListBucketDomains(h.Cfg.QiniuBucket)
	if err != nil {
		return "", fmt.Errorf("获取七牛域名失败: %w", err)
	}
	if len(domains) == 0 {
		return "", fmt.Errorf("bucket %s 未绑定域名", h.Cfg.QiniuBucket)
	}

	qiniuDomainMu.Lock()
	qiniuDomain = domains[0].Domain
	qiniuDomainMu.Unlock()
	logger.Log.Info().Str("domain", domains[0].Domain).Msg("自动发现七牛域名")
	return domains[0].Domain, nil
}

func (h *MediaHandler) Upload(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		logger.Log.Warn().Err(err).Uint("building_id", bid).Msg("上传失败: 未收到文件")
		utils.Error(c, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()

	buf := make([]byte, 512)
	if _, err := file.Read(buf); err != nil {
		logger.Log.Warn().Err(err).Uint("building_id", bid).Msg("上传失败: 无法读取文件头")
		utils.Error(c, http.StatusBadRequest, "无法读取文件")
		return
	}
	file.Seek(0, io.SeekStart)

	mimeType := http.DetectContentType(buf)
	expectedExt, ok := allowedMIMEs[mimeType]
	if !ok {
		logger.Log.Warn().
			Str("mime", mimeType).
			Str("filename", header.Filename).
			Int64("size", header.Size).
			Uint("building_id", bid).
			Msg("上传失败: 不支持的文件类型")
		utils.Error(c, http.StatusBadRequest, "不支持的文件类型，仅允许 jpg/png/gif/mp4/mov")
		return
	}
	mediaType := "image"
	if strings.HasPrefix(mimeType, "video/") {
		mediaType = "video"
	}
	if header.Size > maxSizes[mediaType] {
		limit := maxSizes[mediaType] / 1024 / 1024
		logger.Log.Warn().
			Int64("size", header.Size).
			Str("media_type", mediaType).
			Int64("max", maxSizes[mediaType]).
			Uint("building_id", bid).
			Msg("上传失败: 文件过大")
		utils.Error(c, http.StatusBadRequest, fmt.Sprintf("文件过大，%s 最大 %dMB", mediaType, limit))
		return
	}

	category := c.PostForm("category")
	if category == "" {
		category = "gallery"
	}

	subDir := mediaType + "s"
	if category == "cover" {
		subDir = "covers"
	}

	uuidName := uuid.New().String() + expectedExt
	buildingStr := fmt.Sprintf("%d", bid)
	key := filepath.Join("buildings", buildingStr, "rooms", roomID, subDir, uuidName)
	key = filepath.ToSlash(key)

	media := models.RoomMedia{
		RoomID:   room.ID,
		Type:     mediaType,
		Category: category,
		FilePath: key,
		FileName: header.Filename,
		FileSize: header.Size,
	}
	if err := h.DB.Create(&media).Error; err != nil {
		logger.Log.Error().Err(err).Msg("保存媒体记录到数据库失败")
		utils.Error(c, http.StatusInternalServerError, "保存记录失败")
		return
	}

	if h.useQiniu() {
		if err := h.qiniuUpload(key, file, header.Size); err != nil {
			h.DB.Delete(&media)
			logger.Log.Error().Err(err).Str("key", key).Msg("上传文件到七牛失败")
			utils.Error(c, http.StatusInternalServerError, "上传文件失败")
			return
		}
		logger.Log.Info().
			Uint("media_id", media.ID).
			Uint("room_id", room.ID).
			Uint("building_id", bid).
			Str("key", key).
			Int64("size", header.Size).
			Msg("文件上传到七牛成功")
	} else {
		absPath := filepath.Join(h.Cfg.UploadDir, key)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			h.DB.Delete(&media)
			logger.Log.Error().Err(err).Msg("创建目录失败")
			utils.Error(c, http.StatusInternalServerError, "创建目录失败")
			return
		}
		out, err := os.Create(absPath)
		if err != nil {
			h.DB.Delete(&media)
			logger.Log.Error().Err(err).Msg("保存文件失败")
			utils.Error(c, http.StatusInternalServerError, "保存文件失败")
			return
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			h.DB.Delete(&media)
			logger.Log.Error().Err(err).Msg("写入文件失败")
			utils.Error(c, http.StatusInternalServerError, "写入文件失败")
			return
		}
		logger.Log.Info().
			Uint("media_id", media.ID).
			Uint("room_id", room.ID).
			Uint("building_id", bid).
			Str("file_name", header.Filename).
			Str("type", mediaType).
			Int64("size", header.Size).
			Msg("文件上传到本地成功")
	}
	utils.Created(c, "上传成功", gin.H{"media": media})
}

func (h *MediaHandler) Delete(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	mediaID := c.Param("mediaId")
	var media models.RoomMedia
	if err := h.DB.First(&media, mediaID).Error; err != nil {
		logger.Log.Warn().Str("media_id", mediaID).Msg("删除媒体文件失败: 不存在")
		utils.Error(c, http.StatusNotFound, "媒体文件不存在")
		return
	}
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", media.RoomID, bid).First(&room).Error; err != nil {
		logger.Log.Warn().Uint("media_id", media.ID).Uint("building_id", bid).Msg("删除媒体文件失败: 无权操作")
		utils.Error(c, http.StatusForbidden, "无权操作")
		return
	}

	if h.useQiniu() {
		if err := h.qiniuDelete(media.FilePath); err != nil {
			logger.Log.Error().Err(err).Str("key", media.FilePath).Msg("删除七牛文件失败")
		}
	} else {
		absPath := filepath.Join(h.Cfg.UploadDir, media.FilePath)
		if err := os.Remove(absPath); err != nil {
			logger.Log.Error().Err(err).Str("path", absPath).Msg("删除本地文件失败")
		}
	}
	if err := h.DB.Delete(&media).Error; err != nil {
		logger.Log.Error().Err(err).Uint("media_id", media.ID).Msg("删除媒体记录失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Uint("media_id", media.ID).Uint("room_id", room.ID).Str("file_path", media.FilePath).Msg("媒体文件已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *MediaHandler) UploadCover(c *gin.Context) {
	buildingID, exists := c.Get("building_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	bid, ok := buildingID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}
	var building models.Building
	if err := h.DB.First(&building, bid).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		logger.Log.Warn().Err(err).Uint("building_id", bid).Msg("封面上传失败: 未收到文件")
		utils.Error(c, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()

	buf := make([]byte, 512)
	if _, err := file.Read(buf); err != nil {
		utils.Error(c, http.StatusBadRequest, "无法读取文件")
		return
	}
	file.Seek(0, io.SeekStart)

	mimeType := http.DetectContentType(buf)
	expectedExt, ok := allowedMIMEs[mimeType]
	if !ok || strings.HasPrefix(mimeType, "video/") {
		logger.Log.Warn().
			Str("mime", mimeType).
			Str("filename", header.Filename).
			Uint("building_id", bid).
			Msg("封面上传失败: 不支持的文件类型")
		utils.Error(c, http.StatusBadRequest, "不支持的文件类型，仅允许 jpg/png/gif")
		return
	}
	if header.Size > maxSizes["image"] {
		limit := maxSizes["image"] / 1024 / 1024
		utils.Error(c, http.StatusBadRequest, fmt.Sprintf("图片过大，最大 %dMB", limit))
		return
	}

	uuidName := uuid.New().String() + expectedExt
	buildingStr := fmt.Sprintf("%d", bid)
	key := filepath.ToSlash(filepath.Join("buildings", buildingStr, "cover", uuidName))

	if h.useQiniu() {
		if err := h.qiniuUpload(key, file, header.Size); err != nil {
			logger.Log.Error().Err(err).Str("key", key).Msg("封面上传到七牛失败")
			utils.Error(c, http.StatusInternalServerError, "上传封面失败")
			return
		}
	} else {
		absPath := filepath.Join(h.Cfg.UploadDir, key)
		if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
			logger.Log.Error().Err(err).Msg("创建封面目录失败")
			utils.Error(c, http.StatusInternalServerError, "创建目录失败")
			return
		}
		out, err := os.Create(absPath)
		if err != nil {
			logger.Log.Error().Err(err).Msg("保存封面文件失败")
			utils.Error(c, http.StatusInternalServerError, "保存封面失败")
			return
		}
		defer out.Close()
		if _, err := io.Copy(out, file); err != nil {
			logger.Log.Error().Err(err).Msg("写入封面文件失败")
			utils.Error(c, http.StatusInternalServerError, "写入封面失败")
			return
		}
	}

	if err := h.DB.Model(&building).Update("cover_image", key).Error; err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("更新公寓封面字段失败")
		utils.Error(c, http.StatusInternalServerError, "保存封面失败")
		return
	}
	logger.Log.Info().Uint("building_id", bid).Str("key", key).Msg("公寓封面上传成功")
	utils.Created(c, "封面上传成功", gin.H{"cover_image": key})
}

func (h *MediaHandler) Serve(c *gin.Context) {
	filePath := c.Param("filepath")
	safePath := filepath.Clean(filePath)
	safePath = strings.TrimPrefix(safePath, "/")

	safePath = strings.ReplaceAll(safePath, "\\", "/")
	safePath = strings.TrimLeft(safePath, "/")

	if h.useQiniu() {
		domain, err := h.getDomain()
		if err != nil {
			logger.Log.Error().Err(err).Msg("文件服务: 获取七牛域名失败")
			utils.Error(c, http.StatusNotFound, "文件不存在")
			return
		}
		scheme := "http"
		if h.Cfg.QiniuUseHTTPS {
			scheme = "https"
		}
		url := fmt.Sprintf("%s://%s/%s", scheme, domain, safePath)
		logger.Log.Debug().Str("path", safePath).Str("redirect", url).Msg("文件服务: 重定向到七牛")
		c.Redirect(http.StatusFound, url)
		return
	}

	absPath := filepath.Clean(filepath.Join(h.Cfg.UploadDir, safePath))
	cleanUploadDir := filepath.Clean(h.Cfg.UploadDir)
	if !strings.HasPrefix(absPath, cleanUploadDir) {
		logger.Log.Warn().Str("path", filePath).Msg("文件服务: 非法路径请求")
		utils.Error(c, http.StatusBadRequest, "非法路径")
		return
	}
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		logger.Log.Debug().Str("path", safePath).Msg("文件服务: 文件不存在")
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}
	logger.Log.Debug().Str("path", safePath).Msg("文件服务")
	c.File(absPath)
}
