// Package handlers 处理媒体文件上传、删除、服务等接口，支持本地存储和七牛云存储
package handlers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
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

// MediaHandler 媒体处理器，依赖数据库连接、配置和媒体服务
type MediaHandler struct {
	DB           *gorm.DB
	Cfg          *config.Config
	MediaService *services.MediaService
}

// allowedMIMEs 允许上传的文件 MIME 类型及其对应扩展名
var allowedMIMEs = map[string]string{
	"image/jpeg":      ".jpg",
	"image/png":       ".png",
	"image/gif":       ".gif",
	"video/mp4":       ".mp4",
	"video/quicktime": ".mov",
}

// maxSizes 不同类型媒体的最大文件大小限制
var maxSizes = map[string]int64{
	"image": 10 * 1024 * 1024,
	"video": 200 * 1024 * 1024,
}

// validatedFile 存储文件验证后的元数据
type validatedFile struct {
	MimeType  string
	Ext       string
	MediaType string
	Size      int64
	Filename  string
}

// validateUploadFile 验证上传文件类型和大小，返回文件元数据和句柄
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

// uploadToStorage 上传文件到存储（七牛云或本地）
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

// qiniuUploadURL 根据区域返回七牛云上传地址
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

// getZone 根据区域名称返回七牛云存储区域配置
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

// qiniuDomain 缓存七牛云存储域名，避免重复查询
var qiniuDomain string

// qiniuDomainMu 保护 qiniuDomain 的并发读写安全
var qiniuDomainMu sync.RWMutex

// useQiniu 判断是否配置了七牛云存储
func (h *MediaHandler) useQiniu() bool {
	return h.Cfg.QiniuAccessKey != "" && h.Cfg.QiniuBucket != ""
}

// qiniuMac 创建七牛云鉴权对象
func (h *MediaHandler) qiniuMac() *qiniuAuth.Mac {
	return qiniuAuth.NewMac(h.Cfg.QiniuAccessKey, h.Cfg.QiniuSecretKey)
}

// qiniuConfig 创建七牛云存储配置
func (h *MediaHandler) qiniuConfig() storage.Config {
	return storage.Config{Region: getZone(h.Cfg.QiniuZone), UseHTTPS: h.Cfg.QiniuUseHTTPS}
}

// qiniuUpload 将文件上传到七牛云存储
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

// qiniuDelete 从七牛云存储删除文件
func (h *MediaHandler) qiniuDelete(key string) error {
	cfg := h.qiniuConfig()
	bucketMgr := storage.NewBucketManager(h.qiniuMac(), &cfg)
	return bucketMgr.Delete(h.Cfg.QiniuBucket, key)
}

// getDomain 获取七牛云存储域名（从配置或自动发现）
func (h *MediaHandler) getDomain() (string, error) {
	qiniuDomainMu.RLock()
	d := qiniuDomain
	qiniuDomainMu.RUnlock()
	if d != "" {
		return d, nil
	}

	if h.Cfg.QiniuDomain != "" {
		qiniuDomainMu.Lock()
		qiniuDomain = h.Cfg.QiniuDomain
		qiniuDomainMu.Unlock()
		logger.Log.Info().Str("domain", h.Cfg.QiniuDomain).Msg("使用配置的七牛域名")
		return h.Cfg.QiniuDomain, nil
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

// uploadAndProcess 处理图片（压缩、生成缩略图）并上传
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

// deleteStoredFile 从存储中删除文件（七牛云或本地），失败仅记录日志
func deleteStoredFile(cfg *config.Config, key string) {
	if key == "" {
		return
	}
	if cfg.QiniuAccessKey != "" && cfg.QiniuBucket != "" {
		mac := qiniuAuth.NewMac(cfg.QiniuAccessKey, cfg.QiniuSecretKey)
		qcfg := storage.Config{Region: getZone(cfg.QiniuZone), UseHTTPS: cfg.QiniuUseHTTPS}
		bucketMgr := storage.NewBucketManager(mac, &qcfg)
		if err := bucketMgr.Delete(cfg.QiniuBucket, key); err != nil {
			logger.Log.Warn().Err(err).Str("key", key).Msg("删除七牛文件失败")
		}
		return
	}
	absPath := filepath.Join(cfg.UploadDir, key)
	if err := os.Remove(absPath); err != nil {
		logger.Log.Warn().Err(err).Str("path", absPath).Msg("删除本地文件失败")
	}
}

// deleteFile 从存储中删除文件（七牛云或本地）
func (h *MediaHandler) deleteFile(key string) {
	deleteStoredFile(h.Cfg, key)
}

// urlSafeBase64 七牛风格的 URL-safe Base64 编码（无 padding）
func urlSafeBase64(data []byte) string {
	return strings.TrimRight(base64.URLEncoding.EncodeToString(data), "=")
}

// transcodeKey 生成转码输出 key：在文件名前追加 _720p
func transcodeKey(key string) string {
	ext := filepath.Ext(key)
	return strings.TrimSuffix(key, ext) + "_720p" + ext
}

// pfopTrigger 调用七牛持久化处理接口触发转码，返回 persistentId。
// 使用 QBox 签名直接调用 api.qiniu.com，不依赖 SDK 的 media 包。
func (h *MediaHandler) pfopTrigger(fops, key, notifyURL string) (string, error) {
	form := url.Values{}
	form.Set("bucket", h.Cfg.QiniuBucket)
	form.Set("key", key)
	form.Set("fops", fops)
	if notifyURL != "" {
		form.Set("notifyURL", notifyURL)
	}
	body := form.Encode()
	path := "/pfop"

	req, err := http.NewRequest(http.MethodPost, "https://api.qiniu.com"+path, strings.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	token, err := h.qiniuMac().SignRequest(req)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "QBox "+token)

	client := &http.Client{Timeout: 20 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("pfop 请求失败: %d %s", resp.StatusCode, string(data))
	}
	var ret struct {
		PersistentID string `json:"persistentId"`
	}
	if err := json.Unmarshal(data, &ret); err != nil {
		return "", fmt.Errorf("解析 pfop 响应失败: %v", err)
	}
	if ret.PersistentID == "" {
		return "", fmt.Errorf("pfop 响应缺少 persistentId: %s", string(data))
	}
	return ret.PersistentID, nil
}

// pfopNotifyURL 生成七牛转码回调地址（带校验密钥，防止伪造回调）
func (h *MediaHandler) pfopNotifyURL(c *gin.Context) string {
	base := h.Cfg.QiniuPfopCallbackURL
	if base == "" && c != nil && c.Request != nil {
		scheme := "http"
		if c.Request.TLS != nil || strings.HasPrefix(c.Request.Header.Get("X-Forwarded-Proto"), "https") {
			scheme = "https"
		}
		base = fmt.Sprintf("%s://%s/api/qiniu/pfop-callback", scheme, c.Request.Host)
	}
	if base == "" {
		return ""
	}
	return base + "?secret=" + url.QueryEscape(h.Cfg.QiniuPfopCallbackSecret)
}

// tryTriggerTranscode 视频上传后触发七牛持久化转码（720p H.264），失败不阻塞上传
func (h *MediaHandler) tryTriggerTranscode(c *gin.Context, media *models.RoomMedia) {
	if !h.useQiniu() || !h.Cfg.QiniuPfopEnable || media.Type != "video" || media.Status == "processing" {
		return
	}
	newKey := transcodeKey(media.FilePath)
	saveas := urlSafeBase64([]byte(fmt.Sprintf("%s:%s", h.Cfg.QiniuBucket, newKey)))
	fops := fmt.Sprintf(
		"avthumb/mp4/vcodec/libx264/vb/1500k/acodec/aac/ab/128k/s/1280x720/autoscale/1/movflags/faststart|saveas/%s",
		saveas,
	)
	notifyURL := h.pfopNotifyURL(c)
	pid, err := h.pfopTrigger(fops, media.FilePath, notifyURL)
	if err != nil {
		logger.Log.Warn().Err(err).Uint("media_id", media.ID).Str("key", media.FilePath).Msg("触发视频转码失败，将按原件存储")
		return
	}
	if err := h.MediaService.UpdateMedia(media, map[string]interface{}{"status": "processing", "transcode_id": pid}); err != nil {
		logger.Log.Warn().Err(err).Uint("media_id", media.ID).Msg("更新转码状态失败")
	}
	logger.Log.Info().Uint("media_id", media.ID).Str("key", media.FilePath).Str("persistent_id", pid).Msg("视频转码任务已提交")
}

// PfopCallbackReq 七牛持久化处理回调通知结构
type PfopCallbackReq struct {
	ID          string `json:"id"`
	Code        int    `json:"code"`
	Desc        string `json:"desc"`
	InputBucket string `json:"inputBucket"`
	InputKey    string `json:"inputKey"`
	Items       []struct {
		Code int    `json:"code"`
		Key  string `json:"key"`
		Desc string `json:"desc"`
	} `json:"items"`
}

// PfopCallback 接收七牛转码完成回调：更新媒体记录指向压缩版并删除原片
func (h *MediaHandler) PfopCallback(c *gin.Context) {
	secret := c.Query("secret")
	if h.Cfg.QiniuPfopCallbackSecret == "" || secret != h.Cfg.QiniuPfopCallbackSecret {
		utils.Error(c, http.StatusForbidden, "无效的回调密钥")
		return
	}

	var req PfopCallbackReq
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "回调数据格式错误")
		return
	}
	if req.ID == "" || req.InputKey == "" {
		utils.Error(c, http.StatusBadRequest, "回调缺少必要字段")
		return
	}

	var media models.RoomMedia
	if err := h.DB.Where("transcode_id = ?", req.ID).First(&media).Error; err != nil {
		// 记录不存在（已删除）：尽力清理转码产物，避免孤儿文件
		if req.Code == 0 && len(req.Items) > 0 && req.Items[0].Key != "" {
			h.deleteFile(req.Items[0].Key)
		}
		c.JSON(http.StatusOK, gin.H{"code": 0})
		return
	}

	if req.Code == 0 && len(req.Items) > 0 && req.Items[0].Code == 0 && req.Items[0].Key != "" {
		newKey := req.Items[0].Key
		newSize := int64(0)
		if h.useQiniu() {
			cfg := h.qiniuConfig()
			bm := storage.NewBucketManager(h.qiniuMac(), &cfg)
			if info, e := bm.Stat(h.Cfg.QiniuBucket, newKey); e == nil {
				newSize = info.Fsize
			}
		}
		updates := map[string]interface{}{
			"status":       "ready",
			"file_path":    newKey,
			"file_size":    newSize,
			"transcode_id": "",
		}
		if uerr := h.MediaService.UpdateMedia(&media, updates); uerr != nil {
			logger.Log.Error().Err(uerr).Uint("media_id", media.ID).Msg("转码回调更新记录失败")
			utils.Error(c, http.StatusInternalServerError, "更新失败")
			return
		}
		if req.InputKey != newKey {
			h.deleteFile(req.InputKey)
		}
		logger.Log.Info().Uint("media_id", media.ID).Str("from", req.InputKey).Str("to", newKey).Msg("视频转码完成，原片已删除")
	} else {
		if uerr := h.MediaService.UpdateMedia(&media, map[string]interface{}{"status": "failed", "transcode_id": ""}); uerr != nil {
			logger.Log.Error().Err(uerr).Uint("media_id", media.ID).Msg("更新转码失败状态出错")
		}
		logger.Log.Warn().Uint("media_id", media.ID).Str("key", media.FilePath).Int("code", req.Code).Msg("视频转码失败，保留原片")
	}

	c.JSON(http.StatusOK, gin.H{"code": 0})
}

// Upload 上传媒体文件到房间，支持图片压缩和视频上传
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

	if vf.MediaType == "video" {
		h.tryTriggerTranscode(c, &media)
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

// Delete 删除指定媒体文件及其缩略图
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

// UploadCover 上传公寓封面图片
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

// InitUploadReq 初始化上传请求参数（用于七牛云直传）
type InitUploadReq struct {
	RoomID   string `json:"room_id" form:"room_id"`
	Category string `json:"category" form:"category"`
	Ext      string `json:"ext" form:"ext"`
	FileSize int64  `json:"file_size" form:"file_size"`
}

// GetUploadToken 获取七牛云直传的上传令牌
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
		"token":      upToken,
		"key":        key,
		"upload_url": uploadHost,
		"domain":     domain,
		"cdn_scheme": scheme,
	})
}

// ConfirmUploadReq 确认上传请求参数（用于七牛云直传后确认）
type ConfirmUploadReq struct {
	Key      string `json:"key" binding:"required"`
	Type     string `json:"type" binding:"required"`
	Category string `json:"category"`
	FileName string `json:"file_name"`
	FileSize int64  `json:"file_size"`
}

// ConfirmUpload 确认七牛云直传完成，保存媒体记录
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

	if req.Type == "video" {
		h.tryTriggerTranscode(c, &media)
	}

	logger.Log.Info().
		Uint("media_id", media.ID).
		Uint("room_id", room.ID).
		Uint("building_id", bid).
		Str("key", req.Key).
		Msg("直传文件确认成功")
	utils.Created(c, "上传成功", gin.H{"media": media})
}

// Serve 提供媒体文件服务（支持本地和七牛云重定向）
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
		upstream := fmt.Sprintf("%s://%s/%s", scheme, domain, safePath)
		if h.Cfg.QiniuProxyMode {
			// 代理模式：微信内置浏览器会拦截对 HTTP 非备案 CDN 域名的资源加载，
			// 导致图片加载失败，故由服务端代理回源，浏览器只与本站同源通信。
			logger.Log.Debug().Str("path", safePath).Msg("文件服务: 代理回源")
			h.proxyUpstream(c, upstream, safePath)
		} else {
			// 直连模式：适用于已配置备案 HTTPS CDN 域名、无需代理的场景，
			// 流量直接走 CDN，不占用服务器带宽。
			logger.Log.Debug().Str("path", safePath).Str("redirect", upstream).Msg("文件服务: 重定向到七牛")
			c.Header("Cache-Control", "public, max-age=31536000, immutable")
			c.Redirect(http.StatusFound, upstream)
		}
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

// proxyUpstream 从上游(七牛)拉取文件并透传回客户端：
// 转发 Range 请求头以支持视频拖动播放，超时 30 秒，失败时返回 502。
func (h *MediaHandler) proxyUpstream(c *gin.Context, upstream, safePath string) {
	req, err := http.NewRequestWithContext(c.Request.Context(), http.MethodGet, upstream, nil)
	if err != nil {
		logger.Log.Error().Err(err).Str("url", upstream).Msg("文件服务: 构建上游请求失败")
		utils.Error(c, http.StatusBadGateway, "文件服务不可用")
		return
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; RentalServer/1.0)")
	if r := c.GetHeader("Range"); r != "" {
		req.Header.Set("Range", r)
	}
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		logger.Log.Warn().Err(err).Str("url", upstream).Msg("文件服务: 上游拉取失败")
		utils.Error(c, http.StatusBadGateway, "文件服务不可用")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		logger.Log.Warn().Str("url", upstream).Int("status", resp.StatusCode).Msg("文件服务: 上游返回错误")
		utils.Error(c, http.StatusNotFound, "文件不存在")
		return
	}

	ctype := resp.Header.Get("Content-Type")
	if ctype == "" || strings.HasPrefix(ctype, "text/plain") || strings.HasPrefix(ctype, "application/octet-stream") {
		if t := mimeTypeByExt(safePath); t != "" {
			ctype = t
		}
	}
	c.Header("Content-Type", ctype)
	c.Header("Cache-Control", "public, max-age=31536000, immutable")
	if resp.StatusCode == http.StatusPartialContent {
		if cr := resp.Header.Get("Content-Range"); cr != "" {
			c.Header("Content-Range", cr)
		}
	}
	c.Status(resp.StatusCode)
	if _, err := io.Copy(c.Writer, resp.Body); err != nil {
		logger.Log.Warn().Err(err).Str("url", upstream).Msg("文件服务: 透传中断")
	}
}

func mimeTypeByExt(path string) string {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".jpg", ".jpeg":
		return "image/jpeg"
	case ".png":
		return "image/png"
	case ".gif":
		return "image/gif"
	case ".webp":
		return "image/webp"
	case ".svg":
		return "image/svg+xml"
	case ".bmp":
		return "image/bmp"
	case ".mp4":
		return "video/mp4"
	case ".mov":
		return "video/quicktime"
	case ".webm":
		return "video/webm"
	default:
		return ""
	}
}

// ReDownloadFFmpeg 重新下载 FFmpeg 核心文件
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
