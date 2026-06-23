package handlers

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"

	"rental-server/config"
	"rental-server/models"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
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

func (h *MediaHandler) Upload(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	roomID := c.Param("id")
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", roomID, bid).First(&room).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "房间不存在"})
		return
	}
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请选择文件"})
		return
	}
	defer file.Close()

	buf := make([]byte, 512)
	if _, err := file.Read(buf); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无法读取文件"})
		return
	}
	file.Seek(0, io.SeekStart)

	mimeType := http.DetectContentType(buf)
	ext, ok := allowedMIMEs[mimeType]
	if !ok {
		c.JSON(http.StatusBadRequest, gin.H{"error": "不支持的文件类型，仅允许 jpg/png/gif/mp4/mov"})
		return
	}
	mediaType := "image"
	if strings.HasPrefix(mimeType, "video/") {
		mediaType = "video"
	}
	if header.Size > maxSizes[mediaType] {
		limit := maxSizes[mediaType] / 1024 / 1024
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("文件过大，%s 最大 %dMB", mediaType, limit)})
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

	uuidName := uuid.New().String() + ext
	buildingStr := fmt.Sprintf("%d", bid)
	relPath := filepath.Join("buildings", buildingStr, "rooms", roomID, subDir, uuidName)
	absPath := filepath.Join(h.Cfg.UploadDir, relPath)
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建目录失败"})
		return
	}
	out, err := os.Create(absPath)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存文件失败"})
		return
	}
	defer out.Close()
	if _, err := io.Copy(out, file); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "写入文件失败"})
		return
	}
	media := models.RoomMedia{
		RoomID:   room.ID,
		Type:     mediaType,
		Category: category,
		FilePath: filepath.ToSlash(relPath),
		FileName: header.Filename,
		FileSize: header.Size,
	}
	h.DB.Create(&media)
	c.JSON(http.StatusCreated, gin.H{"media": media})
}

func (h *MediaHandler) Delete(c *gin.Context) {
	buildingID, _ := c.Get("building_id")
	bid := buildingID.(uint)
	mediaID := c.Param("mediaId")
	var media models.RoomMedia
	if err := h.DB.First(&media, mediaID).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "媒体文件不存在"})
		return
	}
	var room models.Room
	if err := h.DB.Where("id = ? AND building_id = ?", media.RoomID, bid).First(&room).Error; err != nil {
		c.JSON(http.StatusForbidden, gin.H{"error": "无权操作"})
		return
	}
	absPath := filepath.Join(h.Cfg.UploadDir, media.FilePath)
	os.Remove(absPath)
	h.DB.Delete(&media)
	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

func (h *MediaHandler) Serve(c *gin.Context) {
	filePath := c.Param("filepath")
	safePath := path.Clean(filePath)
	safePath = strings.TrimPrefix(safePath, "/")
	if strings.Contains(safePath, "..") || strings.HasPrefix(safePath, "/") {
		c.JSON(http.StatusBadRequest, gin.H{"error": "非法路径"})
		return
	}
	absPath := filepath.Join(h.Cfg.UploadDir, safePath)
	if _, err := os.Stat(absPath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{"error": "文件不存在"})
		return
	}
	c.File(absPath)
}
