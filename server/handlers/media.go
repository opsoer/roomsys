package handlers

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	qiniuAuth "github.com/qiniu/go-sdk/v7/auth/qbox"
	"github.com/qiniu/go-sdk/v7/storage"
	"gorm.io/gorm"
)

type MediaHandler struct {
	DB           *gorm.DB
	Cfg          *config.Config
	MediaService *services.MediaService
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

type validatedFile struct {
	MimeType string
	Ext      string
	MediaType string
	Size     int64
	Filename string
}

func validateUploadFile(c *gin.Context, allowVideo bool) (*validatedFile, multipart.File, *multipart.FileHeader, error) {
	file, header, err := c.Request.FormFile("file")
	if err != nil {
		return nil, nil, nil, err
	}

	buf := make([]byte, 512)
	if _, err := file.Read(buf); err != nil {
		file.Close()
		return nil, nil, nil, err
	}
	file.Seek(0, io.SeekStart)

	mimeType := http.DetectContentType(buf)
	expectedExt, ok := allowedMIMEs[mimeType]
	if !ok || (!allowVideo && strings.HasPrefix(mimeType, "video/")) {
		file.Close()
		return nil, nil, nil, fmt.Errorf("不支持的文件类型")
	}

	mediaType := "image"
	if strings.HasPrefix(mimeType, "video/") {
		mediaType = "video"
	}

	if header.Size > maxSizes[mediaType] {
		file.Close()
		return nil, nil, nil, fmt.Errorf("文件过大")
	}

	return &validatedFile{
		MimeType:  mimeType,
		Ext:       expectedExt,
		MediaType: mediaType,
		Size:      header.Size,
		Filename:  header.Filename,
	}, file, header, nil
}

func (h *MediaHandler) uploadToStorage(key string, file io.Reader, size int64) error {
	if h.useQiniu() {
		return h.qiniuUpload(key, file, size)
	}
	absPath := filepath.Join(h.Cfg.UploadDir, key)
	if err := os.MkdirAll(filepath.Dir(absPath), 0755); err != nil {
		return err
	}
	out, err := os.Create(absPath)
	if err != nil {
		return err
	}
	defer out.Close()
	_, err = io.Copy(out, file)
	return err
}

func qiniuUploadURL(zone string) string {
	switch zone {
	case "z0":
		return "https://upload.qiniup.com"
	case "z1":
		return "https://upload-z1.qiniup.com"
	case "z2":
		return "https://upload-z2.qiniup.com"
	case "na0":
		return "https://upload-na0.qiniup.com"
	default:
		return "https://upload-z2.qiniup.com"
	}
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
	extra := storage.PutExtra{
		Params: map[string]string{
			"x-qn-meta-cache-control": "public, max-age=31536000, immutable",
		},
	}
	return formUploader.Put(context.Background(), &ret, upToken, key, reader, size, &extra)
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

func (h *MediaHandler) uploadAndProcess(fileData []byte, vf *validatedFile, key string) (processedSize int64, thumbKey string, err error) {
	processed, thumbnail, pErr := utils.ProcessImageBytes(fileData, vf.Ext)
	if pErr != nil {
		logger.Log.Warn().Err(pErr).Msg("图片处理失败，使用原图")
		processed = fileData
	}

	if err := h.uploadToStorage(key, bytes.NewReader(processed), int64(len(processed))); err != nil {
		return 0, "", err
	}

	if thumbnail != nil {
		thumbName := "thumb_" + filepath.Base(key)
		thumbKey = filepath.ToSlash(filepath.Join(filepath.Dir(key), thumbName))
		if tErr := h.uploadToStorage(thumbKey, bytes.NewReader(thumbnail), int64(len(thumbnail))); tErr != nil {
			logger.Log.Warn().Err(tErr).Msg("上传缩略图失败，不影响主文件")
			thumbKey = ""
		}
	}

	return int64(len(processed)), thumbKey, nil
}

func (h *MediaHandler) deleteFile(key string) {
	if h.useQiniu() {
		if err := h.qiniuDelete(key); err != nil {
			logger.Log.Warn().Err(err).Str("key", key).Msg("删除文件失败")
		}
	} else {
		absPath := filepath.Join(h.Cfg.UploadDir, key)
		if err := os.Remove(absPath); err != nil {
			logger.Log.Warn().Err(err).Str("path", absPath).Msg("删除本地文件失败")
		}
	}
}

func (h *MediaHandler) Upload(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}
	room, err := h.MediaService.GetRoomByID(uint(rid), bid)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}
	vf, file, header, err := validateUploadFile(c, true)
	if err != nil {
		logger.Log.Warn().Err(err).Uint("building_id", bid).Msg("上传失败: " + err.Error())
		utils.Error(c, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()

	category := c.PostForm("category")
	if category == "" {
		category = "gallery"
	}

	if vf.MediaType == "image" && category != "cover" {
		count, _ := h.MediaService.CountMediaByRoomAndType(room.ID, "image")
		if count >= 10 {
			file.Close()
			utils.Error(c, http.StatusBadRequest, "每个房间最多允许10张照片")
			return
		}
	}
	if vf.MediaType == "video" {
		count, _ := h.MediaService.CountMediaByRoomAndType(room.ID, "video")
		if count >= 2 {
			file.Close()
			utils.Error(c, http.StatusBadRequest, "每个房间最多允许2个视频")
			return
		}
	}

	subDir := vf.MediaType + "s"
	if category == "cover" {
		subDir = "covers"

		oldMedias, err := h.MediaService.GetMediaByRoomAndCategory(room.ID, category)
		if err == nil {
			for _, oldMedia := range oldMedias {
				h.deleteFile(oldMedia.FilePath)
				if oldMedia.ThumbnailPath != "" {
					h.deleteFile(oldMedia.ThumbnailPath)
				}
				if delErr := h.MediaService.DeleteMedia(&oldMedia); delErr != nil {
					logger.Log.Error().Err(delErr).Uint("media_id", oldMedia.ID).Msg("删除旧封面记录失败")
				} else {
					logger.Log.Info().Uint("media_id", oldMedia.ID).Str("key", oldMedia.FilePath).Msg("已删除旧封面")
				}
			}
		}
	}

	uuidName := uuid.New().String() + vf.Ext
	building, _ := h.MediaService.GetBuildingByID(bid)
	nameSlug := utils.Slugify(building.Name)
	buildingStr := fmt.Sprintf("%d_%s", bid, nameSlug)
	roomStr := fmt.Sprintf("%d_%s-%s", room.ID, room.Floor, room.RoomNumber)
	key := filepath.ToSlash(filepath.Join("buildings", buildingStr, "rooms", roomStr, subDir, uuidName))

	var thumbKey string
	var processedSize int64

	if vf.MediaType == "image" {
		fileData, rErr := io.ReadAll(file)
		if rErr != nil {
			utils.Error(c, http.StatusInternalServerError, "读取文件失败")
			return
		}
		processedSize, thumbKey, err = h.uploadAndProcess(fileData, vf, key)
		if err != nil {
			logger.Log.Error().Err(err).Str("key", key).Msg("上传文件失败")
			utils.Error(c, http.StatusInternalServerError, "上传文件失败")
			return
		}
	} else {
		if err := h.uploadToStorage(key, file, header.Size); err != nil {
			logger.Log.Error().Err(err).Str("key", key).Msg("上传文件失败")
			utils.Error(c, http.StatusInternalServerError, "上传文件失败")
			return
		}
		processedSize = header.Size
	}

	media := models.RoomMedia{
		RoomID:        room.ID,
		Type:          vf.MediaType,
		Category:      category,
		FilePath:      key,
		ThumbnailPath: thumbKey,
		FileName:      header.Filename,
		FileSize:      header.Size,
	}
	if err := h.MediaService.CreateMedia(&media); err != nil {
		h.deleteFile(key)
		if thumbKey != "" {
			h.deleteFile(thumbKey)
		}
		logger.Log.Error().Err(err).Msg("保存媒体记录到数据库失败")
		utils.Error(c, http.StatusInternalServerError, "保存记录失败")
		return
	}

	logger.Log.Info().
		Uint("media_id", media.ID).
		Uint("room_id", room.ID).
		Uint("building_id", bid).
		Str("key", key).
		Int64("size", header.Size).
		Int64("compressed_size", processedSize).
		Msg("文件上传成功")
	utils.Created(c, "上传成功", gin.H{"media": media})
}
func (h *MediaHandler) Delete(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	mediaID := c.Param("mediaId")
	mid, err := strconv.ParseUint(mediaID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的媒体ID")
		return
	}
	media, err := h.MediaService.GetMediaByID(uint(mid))
	if err != nil {
		logger.Log.Warn().Str("media_id", mediaID).Msg("删除媒体文件失败: 不存在")
		utils.Error(c, http.StatusNotFound, "媒体文件不存在")
		return
	}
	room, err := h.MediaService.GetRoomByID(media.RoomID, bid)
	if err != nil {
		logger.Log.Warn().Uint("media_id", media.ID).Uint("building_id", bid).Msg("删除媒体文件失败: 无权操作")
		utils.Error(c, http.StatusForbidden, "无权操作")
		return
	}

	h.deleteFile(media.FilePath)
	if media.ThumbnailPath != "" {
		h.deleteFile(media.ThumbnailPath)
	}

	if err := h.MediaService.DeleteMedia(media); err != nil {
		logger.Log.Error().Err(err).Uint("media_id", media.ID).Msg("删除媒体记录失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	logger.Log.Info().Uint("media_id", media.ID).Uint("room_id", room.ID).Str("file_path", media.FilePath).Msg("媒体文件已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *MediaHandler) UploadCover(c *gin.Context) {
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

	if building.CoverImage != "" {
		h.deleteFile(building.CoverImage)
	}

	vf, file, _, err := validateUploadFile(c, false)
	if err != nil {
		logger.Log.Warn().Err(err).Uint("building_id", bid).Msg("封面上传失败: " + err.Error())
		utils.Error(c, http.StatusBadRequest, "请选择文件")
		return
	}
	defer file.Close()

	fileData, err := io.ReadAll(file)
	if err != nil {
		utils.Error(c, http.StatusInternalServerError, "读取文件失败")
		return
	}

	uuidName := uuid.New().String() + vf.Ext
	nameSlug := utils.Slugify(building.Name)
	buildingStr := fmt.Sprintf("%d_%s", bid, nameSlug)
	key := filepath.ToSlash(filepath.Join("buildings", buildingStr, "cover", uuidName))

	if vf.MediaType == "image" {
		processed, pErr := utils.ProcessImageNoThumb(fileData)
		if pErr == nil {
			fileData = processed
		} else {
			logger.Log.Warn().Err(pErr).Msg("图片处理失败，使用原图")
		}
	}

	if err := h.uploadToStorage(key, bytes.NewReader(fileData), int64(len(fileData))); err != nil {
		logger.Log.Error().Err(err).Str("key", key).Msg("封面上传失败")
		utils.Error(c, http.StatusInternalServerError, "上传封面失败")
		return
	}

	if err := h.MediaService.UpdateBuildingCover(building.ID, key); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("更新公寓封面字段失败")
		utils.Error(c, http.StatusInternalServerError, "保存封面失败")
		return
	}
	logger.Log.Info().Uint("building_id", bid).Str("key", key).Msg("公寓封面上传成功")
	utils.Created(c, "封面上传成功", gin.H{"cover_image": key})
}

type InitUploadReq struct {
	RoomID   string `json:"room_id" form:"room_id"`
	Category string `json:"category" form:"category"`
	Ext      string `json:"ext" form:"ext"`
	FileSize int64  `json:"file_size" form:"file_size"`
}

func (h *MediaHandler) GetUploadToken(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	if !h.useQiniu() {
		utils.Error(c, http.StatusBadRequest, "未配置七牛云存储")
		return
	}

	var req InitUploadReq
	if err := c.ShouldBindQuery(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}

	var room *models.Room
	if req.RoomID != "" {
		rid, pErr := strconv.ParseUint(req.RoomID, 10, 32)
		if pErr != nil {
			utils.Error(c, http.StatusBadRequest, "无效的房间ID")
			return
		}
		var rErr error
		room, rErr = h.MediaService.GetRoomByID(uint(rid), bid)
		if rErr != nil {
			utils.Error(c, http.StatusNotFound, "房间不存在")
			return
		}
	}

	if req.Ext == "" {
		req.Ext = ".jpg"
	}
	if !strings.HasPrefix(req.Ext, ".") {
		req.Ext = "." + req.Ext
	}

	subDir := "images"
	uuidName := uuid.New().String() + req.Ext
	building, bErr := h.MediaService.GetBuildingByID(bid)
	if bErr != nil {
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	nameSlug := utils.Slugify(building.Name)
	buildingStr := fmt.Sprintf("%d_%s", bid, nameSlug)
	var roomStr string
	if room != nil {
		roomStr = fmt.Sprintf("%d_%s-%s", room.ID, room.Floor, room.RoomNumber)
	} else {
		roomStr = req.RoomID
	}
	key := filepath.ToSlash(filepath.Join("buildings", buildingStr, "rooms", roomStr, subDir, uuidName))

	putPolicy := storage.PutPolicy{
		Scope:   fmt.Sprintf("%s:%s", h.Cfg.QiniuBucket, key),
		Expires: 3600,
	}
	upToken := putPolicy.UploadToken(h.qiniuMac())

	uploadHost := qiniuUploadURL(h.Cfg.QiniuZone)

	domain, dErr := h.getDomain()
	if dErr != nil {
		utils.Error(c, http.StatusInternalServerError, "获取域名失败")
		return
	}
	scheme := "http"
	if h.Cfg.QiniuUseHTTPS {
		scheme = "https"
	}

	utils.Success(c, gin.H{
		"token":       upToken,
		"key":         key,
		"upload_url":  uploadHost,
		"domain":      domain,
		"cdn_scheme":  scheme,
	})
}

type ConfirmUploadReq struct {
	Key       string `json:"key" binding:"required"`
	Type      string `json:"type" binding:"required"`
	Category  string `json:"category"`
	FileName  string `json:"file_name"`
	FileSize  int64  `json:"file_size"`
}

func (h *MediaHandler) ConfirmUpload(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	roomID := c.Param("id")
	rid, err := strconv.ParseUint(roomID, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的房间ID")
		return
	}
	room, err := h.MediaService.GetRoomByID(uint(rid), bid)
	if err != nil {
		utils.Error(c, http.StatusNotFound, "房间不存在")
		return
	}

	var req ConfirmUploadReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Category == "" {
		req.Category = "gallery"
	}

	if req.Type == "image" && req.Category != "cover" {
		count, _ := h.MediaService.CountMediaByRoomAndType(room.ID, "image")
		if count >= 10 {
			utils.Error(c, http.StatusBadRequest, "每个房间最多允许10张照片")
			return
		}
	}
	if req.Type == "video" {
		count, _ := h.MediaService.CountMediaByRoomAndType(room.ID, "video")
		if count >= 2 {
			utils.Error(c, http.StatusBadRequest, "每个房间最多允许2个视频")
			return
		}
	}

	media := models.RoomMedia{
		RoomID:   room.ID,
		Type:     req.Type,
		Category: req.Category,
		FilePath: req.Key,
		FileName: req.FileName,
		FileSize: req.FileSize,
	}
	if err := h.MediaService.CreateMedia(&media); err != nil {
		logger.Log.Error().Err(err).Msg("保存媒体记录到数据库失败")
		utils.Error(c, http.StatusInternalServerError, "保存记录失败")
		return
	}

	logger.Log.Info().
		Uint("media_id", media.ID).
		Uint("room_id", room.ID).
		Uint("building_id", bid).
		Str("key", req.Key).
		Msg("直传文件确认成功")
	utils.Created(c, "上传成功", gin.H{"media": media})
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
		c.Header("Cache-Control", "public, max-age=31536000, immutable")
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

func (h *MediaHandler) ReDownloadFFmpeg(c *gin.Context) {
	dir := filepath.Join(h.Cfg.UploadDir, "ffmpeg")
	if err := os.MkdirAll(dir, 0750); err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建目录失败")
		return
	}

	baseURL := "https://cdn.jsdelivr.net/npm/@ffmpeg/core@0.12.10/dist/esm"
	files := []string{"ffmpeg-core.js", "ffmpeg-core.wasm"}
	client := &http.Client{Timeout: 120 * time.Second}

	var failed []string
	for _, f := range files {
		url := baseURL + "/" + f
		resp, err := client.Get(url)
		if err != nil {
			logger.Log.Warn().Err(err).Str("url", url).Msg("下载 FFmpeg core 失败")
			failed = append(failed, f)
			continue
		}
		out, err := os.Create(filepath.Join(dir, f))
		if err != nil {
			resp.Body.Close()
			failed = append(failed, f)
			continue
		}
		_, err = io.Copy(out, resp.Body)
		resp.Body.Close()
		out.Close()
		if err != nil {
			os.Remove(filepath.Join(dir, f))
			failed = append(failed, f)
		}
	}

	if len(failed) > 0 {
		utils.Error(c, http.StatusInternalServerError, "部分文件下载失败")
		return
	}
	utils.Success(c, gin.H{"message": "FFmpeg 核心文件下载完成"})
}
