// Package handlers 处理公寓楼栋相关接口，包括公寓的增删改查、套餐升级等
package handlers

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"rental-server/config"
	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// BuildingHandler 公寓处理器，依赖数据库连接和公寓服务
type BuildingHandler struct {
	DB              *gorm.DB
	Cfg             *config.Config
	BuildingService *services.BuildingService
}

// defaultContractDurationYears 合同默认期限为 1 年
const defaultContractDurationYears = 1

// CreateBuildingReq 创建公寓请求参数
type CreateBuildingReq struct {
	Name         string `json:"name" binding:"required"`
	Package      string `json:"package"`
	ContractDate string `json:"contract_date"`
	District     string `json:"district"`
	Street       string `json:"street"`
	Village      string `json:"village"`
	BuildingNo   string `json:"building_no"`
	Description  string `json:"description"`
	Landlords    []struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"landlords"`
}

// UpdateBuildingReq 更新公寓信息请求参数
type UpdateBuildingReq struct {
	Name         string `json:"name"`
	Package      string `json:"package"`
	ContractDate string `json:"contract_date"`
	District     string `json:"district"`
	Street       string `json:"street"`
	Village      string `json:"village"`
	BuildingNo   string `json:"building_no"`
	CoverImage   string `json:"cover_image"`
	Description  string `json:"description"`
	Status       string `json:"status"`
	Landlords    []struct {
		Name  string `json:"name"`
		Phone string `json:"phone"`
	} `json:"landlords"`
}

// Create 创建新公寓，包含名称查重和房东信息录入
func (h *BuildingHandler) Create(c *gin.Context) {
	var req CreateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Msg("创建公寓请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	userID, exists := c.Get("user_id")
	if !exists {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	uid, ok := userID.(uint)
	if !ok {
		utils.Error(c, http.StatusInternalServerError, "服务器错误")
		return
	}

	pkg := req.Package
	if pkg != "basic" && pkg != "full" {
		pkg = "basic"
	}
	building := models.Building{
		Name:         req.Name,
		Package:      pkg,
		ContractDate: req.ContractDate,
		District:     req.District,
		Street:       req.Street,
		Village:      req.Village,
		BuildingNo:   req.BuildingNo,
		Description:  req.Description,
		Status:       "active",
		CreatedBy:    uid,
	}
	if req.ContractDate != "" {
		if cd, err := time.Parse("2006-01-02", req.ContractDate); err == nil {
			building.ExpiredAt = cd.AddDate(defaultContractDurationYears, 0, 0).Format("2006-01-02")
		}
	}
	exists, err := h.BuildingService.ExistsByName(req.Name)
	if err != nil {
		logger.Log.Error().Err(err).Msg("检查公寓名称失败")
		utils.Error(c, http.StatusInternalServerError, "创建公寓失败")
		return
	}
	if exists {
		suggestedName, err := h.BuildingService.GenerateSuggestedName(req.Name)
		if err != nil {
			logger.Log.Warn().Str("name", req.Name).Msg("生成建议名称失败")
			utils.Error(c, http.StatusConflict, "公寓名称已存在")
			return
		}
		logger.Log.Warn().Str("name", req.Name).Str("suggested", suggestedName).Msg("创建公寓失败: 名称已存在")
		c.JSON(http.StatusConflict, utils.APIResponse{
			Code:    utils.CodeNameConflict,
			Message: "公寓名称已存在",
			Data:    gin.H{"suggested_name": suggestedName},
		})
		return
	}
	if err := h.BuildingService.Create(&building); err != nil {
		logger.Log.Error().Err(err).Str("name", req.Name).Msg("创建公寓数据库失败")
		utils.Error(c, http.StatusInternalServerError, "创建公寓失败")
		return
	}
	// 写入入驻记录（入驻日期 → 首次到期日期）
	h.BuildingService.RecordJoin(building.ID, building.ContractDate, building.ExpiredAt)
	for _, l := range req.Landlords {
		ll := models.BuildingLandlord{
			BuildingID: building.ID,
			Name:       l.Name,
			Phone:      l.Phone,
		}
		if err := h.BuildingService.CreateLandlord(&ll); err != nil {
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("创建房东信息失败")
			utils.Error(c, http.StatusInternalServerError, "创建公寓失败")
			return
		}
	}
	logger.Log.Info().Uint("building_id", building.ID).Str("name", building.Name).Uint("created_by", uid).Msg("公寓创建成功")
	utils.Created(c, "公寓创建成功", gin.H{"building": building})
}

// Update 更新公寓信息，支持修改名称、地址、封面等
func (h *BuildingHandler) Update(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	building, err := h.BuildingService.GetByID(uint(buildingID))
	if err != nil {
		logger.Log.Warn().Str("id", id).Msg("更新公寓失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	var req UpdateBuildingReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", building.ID).Msg("更新公寓请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Package != "" {
		if req.Package == "basic" || req.Package == "full" {
			updates["package"] = req.Package
		}
	}
	if req.ContractDate != "" {
		updates["contract_date"] = req.ContractDate
		if cd, err := time.Parse("2006-01-02", req.ContractDate); err == nil {
			updates["expired_at"] = cd.AddDate(defaultContractDurationYears, 0, 0).Format("2006-01-02")
		}
	}
	if req.District != "" {
		updates["district"] = req.District
	}
	if req.Street != "" {
		updates["street"] = req.Street
	}
	if req.Village != "" {
		updates["village"] = req.Village
	}
	if req.BuildingNo != "" {
		updates["building_no"] = req.BuildingNo
	}
	if req.CoverImage != "" {
		updates["cover_image"] = req.CoverImage
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.Status != "" {
		updates["status"] = req.Status
	}

	if req.Name != "" && req.Name != building.Name {
		exists, err := h.BuildingService.ExistsByName(req.Name)
		if err != nil {
			logger.Log.Error().Err(err).Msg("检查公寓名称失败")
			utils.Error(c, http.StatusInternalServerError, "更新失败")
			return
		}
		if exists {
			suggestedName, err := h.BuildingService.GenerateSuggestedName(req.Name)
			if err != nil {
				utils.Error(c, http.StatusConflict, "公寓名称已存在")
				return
			}
			c.JSON(http.StatusConflict, utils.APIResponse{
				Code:    utils.CodeNameConflict,
				Message: "公寓名称已存在",
				Data:    gin.H{"suggested_name": suggestedName},
			})
			return
		}
	}
	if len(updates) > 0 {
		if err := h.BuildingService.Update(building.ID, updates); err != nil {
			logger.Log.Error().Err(err).Uint("building_id", building.ID).Msg("更新公寓数据库失败")
			utils.Error(c, http.StatusConflict, "更新失败")
			return
		}
	}

	if len(req.Landlords) > 0 {
		h.BuildingService.DeleteLandlords(building.ID)
		for _, l := range req.Landlords {
			ll := models.BuildingLandlord{
				BuildingID: building.ID,
				Name:       l.Name,
				Phone:      l.Phone,
			}
			h.BuildingService.CreateLandlord(&ll)
		}
	}

	logger.Log.Info().Uint("building_id", building.ID).Msg("公寓更新成功")
	utils.SuccessWithMsg(c, "更新成功", nil)
}

// Delete 删除指定公寓（存在活跃合同时禁止删除）
func (h *BuildingHandler) Delete(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	if err := h.BuildingService.Delete(uint(buildingID)); err != nil {
		if err.Error() == "active_contracts_exist" {
			logger.Log.Warn().Str("id", id).Msg("删除公寓失败: 存在活跃合同")
			utils.ErrorWithCode(c, http.StatusConflict, utils.CodeActiveContract, "该公寓存在活跃合同，无法删除")
			return
		}
		logger.Log.Error().Err(err).Str("id", id).Msg("删除公寓失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	// 删除该公寓下所有房间的媒体物理文件（封面和房间图片由 MediaHandler 各自管理）
	var mediaPaths []string
	h.DB.Table("room_media").
		Joins("JOIN rooms ON rooms.id = room_media.room_id").
		Where("rooms.building_id = ?", buildingID).
		Pluck("room_media.file_path", &mediaPaths)
	var thumbPaths []string
	h.DB.Table("room_media").
		Joins("JOIN rooms ON rooms.id = room_media.room_id").
		Where("rooms.building_id = ?", buildingID).
		Pluck("room_media.thumbnail_path", &thumbPaths)
	for _, p := range append(mediaPaths, thumbPaths...) {
		if p != "" {
			deleteStoredFile(h.Cfg, p)
		}
	}
	logger.Log.Info().Str("id", id).Msg("公寓已删除")
	utils.SuccessWithMsg(c, "删除成功", nil)
}

// UpgradePackage 升级公寓套餐（basic/full）
func (h *BuildingHandler) UpgradePackage(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	var req struct {
		Package string `json:"package" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Package != "basic" && req.Package != "full" {
		utils.Error(c, http.StatusBadRequest, "无效的套餐类型")
		return
	}
	if err := h.BuildingService.UpgradePackage(uint(buildingID), req.Package); err != nil {
		logger.Log.Error().Err(err).Str("id", id).Msg("升级套餐失败")
		utils.Error(c, http.StatusInternalServerError, "升级失败")
		return
	}
	logger.Log.Info().Str("id", id).Str("package", req.Package).Msg("套餐升级成功")
	utils.SuccessWithMsg(c, "升级成功", nil)
}

// List 获取公寓列表，支持状态和关键词筛选（管理端，包含不可见公寓）
func (h *BuildingHandler) List(c *gin.Context) {
	status := c.Query("status")
	keyword := c.Query("keyword")
	page, size := utils.ParsePage(c)
	lastID, _ := strconv.Atoi(c.Query("last_id"))
	buildings, total, err := h.BuildingService.List(services.BuildingListFilter{Status: status, Keyword: keyword}, page, lastID, size, true)
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询公寓列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"buildings": buildings, "total": total, "page": page, "size": size})
}

// ListPublic 获取公寓列表（公开端，排除不可见公寓），支持位置/价格区间/户型筛选并记录访问。
// 位置、价格、户型之间为 AND 关系；价格与户型要求公寓下存在满足条件的可见房间
// （未出租或即将到期），全部条件都不满足时返回空列表。
func (h *BuildingHandler) ListPublic(c *gin.Context) {
	page, size := utils.ParsePage(c)
	lastID, _ := strconv.Atoi(c.Query("last_id"))
	minPrice, _ := strconv.ParseFloat(c.Query("min_price"), 64)
	maxPrice, _ := strconv.ParseFloat(c.Query("max_price"), 64)
	filter := services.BuildingListFilter{
		District: c.Query("district"),
		Street:   c.Query("street"),
		Village:  c.Query("village"),
		MinPrice: minPrice,
		MaxPrice: maxPrice,
		Layout:   c.Query("layout"),
	}
	buildings, total, err := h.BuildingService.List(filter, page, lastID, size, false)
	if err != nil {
		logger.Log.Error().Err(err).Msg("查询公寓列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	go utils.RecordPageView(h.DB, "building_list", 0, 0, utils.GetRealIP(c))
	utils.Success(c, gin.H{"buildings": buildings, "total": total, "page": page, "size": size})
}

// ListLocations 公开端位置聚合：返回所有可见公寓实际使用的 区域→街道→村/小区 数据，
// 供租客端筛选下拉使用，保证筛选选项与真实房源一致（管理端录入时允许自由输入，可能超出静态列表）。
func (h *BuildingHandler) ListLocations(c *gin.Context) {
	today := utils.Now().Format("2006-01-02")
	var rows []struct {
		District string `gorm:"column:district"`
		Street   string `gorm:"column:street"`
		Village  string `gorm:"column:village"`
	}
	if err := h.DB.Table("buildings").
		Select("district, street, village").
		Where("deleted_at IS NULL AND status <> ? AND (expired_at = '' OR expired_at IS NULL OR expired_at >= ?)",
			services.BuildingStatusHidden, today).
		Where("district <> '' AND street <> '' AND village <> ''").
		Find(&rows).Error; err != nil {
		logger.Log.Error().Err(err).Msg("查询位置聚合失败")
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}

	type streetNode struct {
		Name     string   `json:"name"`
		Villages []string `json:"villages"`
	}
	type districtNode struct {
		Name    string       `json:"name"`
		Streets []streetNode `json:"streets"`
	}

	villageSet := map[string]map[string]map[string]bool{}
	for _, r := range rows {
		if villageSet[r.District] == nil {
			villageSet[r.District] = map[string]map[string]bool{}
		}
		if villageSet[r.District][r.Street] == nil {
			villageSet[r.District][r.Street] = map[string]bool{}
		}
		villageSet[r.District][r.Street][r.Village] = true
	}

	districtNames := make([]string, 0, len(villageSet))
	for d := range villageSet {
		districtNames = append(districtNames, d)
	}
	sort.Strings(districtNames)
	locations := make([]districtNode, 0, len(districtNames))
	for _, d := range districtNames {
		streetNames := make([]string, 0, len(villageSet[d]))
		for st := range villageSet[d] {
			streetNames = append(streetNames, st)
		}
		sort.Strings(streetNames)
		node := districtNode{Name: d, Streets: make([]streetNode, 0, len(streetNames))}
		for _, st := range streetNames {
			villages := make([]string, 0, len(villageSet[d][st]))
			for v := range villageSet[d][st] {
				villages = append(villages, v)
			}
			sort.Strings(villages)
			node.Streets = append(node.Streets, streetNode{Name: st, Villages: villages})
		}
		locations = append(locations, node)
	}
	utils.Success(c, gin.H{"locations": locations})
}

// GetPublic 获取公寓详情（公开端），含统计信息并记录访问
func (h *BuildingHandler) GetPublic(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	visible, err := h.BuildingService.IsVisible(uint(buildingID))
	if err != nil || !visible {
		logger.Log.Warn().Str("id", id).Msg("获取公寓详情失败: 公寓不存在或不可见")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	building, err := h.BuildingService.GetWithStats(uint(buildingID))
	if err != nil {
		logger.Log.Warn().Str("id", id).Msg("获取公寓详情失败: 公寓不存在")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	go utils.RecordPageView(h.DB, "building_detail", uint(buildingID), uint(buildingID), utils.GetRealIP(c))
	utils.Success(c, gin.H{"building": building})
}

// GetRooms 获取公寓的房间列表，支持楼层、户型、状态筛选
func (h *BuildingHandler) GetRooms(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	visible, err := h.BuildingService.IsVisible(uint(buildingID))
	if err != nil || !visible {
		logger.Log.Warn().Str("id", id).Msg("获取房间列表失败: 公寓不存在或不可见")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	requestedStatus := c.Query("status")
	page, size := utils.ParsePage(c)
	lastID, _ := strconv.Atoi(c.Query("last_id"))

	reservedRoomIDs := h.DB.Table("rental_contracts").Select("room_id").Where("status = ?", "reserved")
	query := h.DB.Where("building_id = ? AND status NOT IN ? AND id NOT IN (?)", buildingID, []string{"reserved"}, reservedRoomIDs)

	if lastID > 0 {
		query = query.Where("id > ?", lastID)
	}

	// 已登录且为该公寓的管理员（或超级管理员）可见全部房间；
	// 未登录或其它公寓管理员仅可见「未出租」和「即将到期」的房间
	canViewAll := false
	authHeader := c.GetHeader("Authorization")
	if strings.HasPrefix(authHeader, "Bearer ") {
		if claims, err := utils.ParseToken(strings.TrimPrefix(authHeader, "Bearer "), h.Cfg.JWTSecret); err == nil {
			if claims.Role == "super_admin" || (claims.BuildingID > 0 && claims.BuildingID == uint(buildingID)) {
				canViewAll = true
			}
		}
	}
	if !canViewAll {
		today := utils.Now().Format("2006-01-02")
		expiringBefore := utils.Now().AddDate(0, 0, 30).Format("2006-01-02")
		query = query.Where(
			"(status = ? OR (status = ? AND id IN (SELECT room_id FROM rental_contracts WHERE status = ? AND end_date >= ? AND end_date < ?)))",
			"vacant", "rented", "active", today, expiringBefore,
		)
	}

	if floor := c.Query("floor"); floor != "" {
		query = query.Where("floor = ?", floor)
	}
	if layout := c.Query("layout"); layout != "" {
		query = query.Where("layout = ?", layout)
	}

	// 状态筛选下沉到 SQL，保证 total 与分页结果一致（避免动态状态在内存过滤后 total 虚高导致前端无限加载）
	if requestedStatus != "" {
		today := utils.Now().Format("2006-01-02")
		thirtyDaysLater := utils.Now().AddDate(0, 0, 30).Format("2006-01-02")
		switch requestedStatus {
		case "vacant":
			query = query.Where("status = ?", "vacant")
		case "expiring":
			query = query.Where("status = ? AND id IN (SELECT room_id FROM rental_contracts WHERE status = ? AND end_date != '' AND end_date >= ? AND end_date < ?)",
				"rented", "active", today, thirtyDaysLater)
		case "rented":
			query = query.Where("status = ? AND id NOT IN (SELECT room_id FROM rental_contracts WHERE status = ? AND end_date != '' AND end_date < ?)",
				"rented", "active", thirtyDaysLater)
		case "reserved":
			// 公开端基础查询已排除已预订房间
			query = query.Where("1 = 0")
		}
	}

	var total int64
	if lastID == 0 {
		if err := query.Model(&models.Room{}).Count(&total).Error; err != nil {
			logger.Log.Error().Err(err).Uint("building_id", uint(buildingID)).Msg("查询房间总数失败")
			utils.Error(c, http.StatusInternalServerError, "查询失败")
			return
		}
	} else {
		total = -1
	}

	var rooms []models.Room
	q := query.Preload("Media").Order("id ASC")
	if lastID > 0 {
		if err := q.Limit(size).Find(&rooms).Error; err != nil {
			logger.Log.Error().Err(err).Uint("building_id", uint(buildingID)).Msg("查询房间列表失败")
			utils.Error(c, http.StatusInternalServerError, "查询失败")
			return
		}
	} else {
		if err := q.Offset((page - 1) * size).Limit(size).Find(&rooms).Error; err != nil {
			logger.Log.Error().Err(err).Uint("building_id", uint(buildingID)).Msg("查询房间列表失败")
			utils.Error(c, http.StatusInternalServerError, "查询失败")
			return
		}
	}

	roomIDs := make([]uint, len(rooms))
	for i, r := range rooms {
		roomIDs[i] = r.ID
	}
	var contracts []models.RentalContract
	if len(roomIDs) > 0 {
		h.DB.Where("room_id IN ? AND status = ?", roomIDs, "active").Find(&contracts)
	}
	contractMap := make(map[uint]string)
	for _, ct := range contracts {
		contractMap[ct.RoomID] = ct.EndDate
	}

	type RoomWithThumbnail struct {
		models.Room
		Thumbnail string `json:"thumbnail"`
		EndDate   string `json:"end_date"`
	}
	var result []RoomWithThumbnail
	for _, r := range rooms {
		thumb := ""
		for _, m := range r.Media {
			if m.Category == "cover" && m.Type == "image" {
				if m.ThumbnailPath != "" {
					thumb = m.ThumbnailPath
				} else {
					thumb = m.FilePath
				}
				break
			}
		}
		if thumb == "" {
			for _, m := range r.Media {
				if m.Type == "image" {
					if m.ThumbnailPath != "" {
						thumb = m.ThumbnailPath
					} else {
						thumb = m.FilePath
					}
					break
				}
			}
		}
		endDate := contractMap[r.ID]
		dynStatus := utils.DynamicRoomStatus(r.Status, endDate)
		r.Status = dynStatus
		if requestedStatus != "" && dynStatus != requestedStatus {
			continue
		}
		result = append(result, RoomWithThumbnail{Room: r, Thumbnail: thumb, EndDate: endDate})
	}
	utils.Success(c, gin.H{"rooms": result, "total": total, "page": page, "size": size})
}

// MyBuilding 获取当前管理员所属的公寓信息
func (h *BuildingHandler) MyBuilding(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	building, err := h.BuildingService.GetWithStats(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取公寓信息失败")
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	utils.Success(c, gin.H{"building": building})
}

// UpdateMyBuilding 更新当前管理员所属的公寓信息
func (h *BuildingHandler) UpdateMyBuilding(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		Name        string `json:"name"`
		Description string `json:"description"`
		CoverImage  string `json:"cover_image"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	updates := map[string]interface{}{}
	if req.Name != "" {
		updates["name"] = req.Name
	}
	if req.Description != "" {
		updates["description"] = req.Description
	}
	if req.CoverImage != "" {
		updates["cover_image"] = req.CoverImage
	}
	if err := h.BuildingService.Update(bid, updates); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("更新公寓信息失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	utils.SuccessWithMsg(c, "更新成功", nil)
}

// Renew 公寓续约（超级管理员）：更新到期日期、恢复可见状态并完成相关待办
func (h *BuildingHandler) Renew(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	var req struct {
		ExpiredAt string `json:"expired_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	operator := c.GetString("username")
	if operator == "" {
		if uid, err := utils.GetUserID(c); err == nil {
			operator = fmt.Sprintf("uid:%d", uid)
		}
	}
	if err := h.BuildingService.Renew(uint(buildingID), req.ExpiredAt, operator); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	logger.Log.Info().Uint("building_id", uint(buildingID)).Str("expired_at", req.ExpiredAt).Msg("公寓续约成功")
	utils.SuccessWithMsg(c, "续约成功，公寓已恢复展示", nil)
}

// Renewals 获取公寓入驻/续约历史（超级管理员）
func (h *BuildingHandler) Renewals(c *gin.Context) {
	id := c.Param("id")
	buildingID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的公寓ID")
		return
	}
	if _, err := h.BuildingService.GetByID(uint(buildingID)); err != nil {
		utils.Error(c, http.StatusNotFound, "公寓不存在")
		return
	}
	records, err := h.BuildingService.ListRenewals(uint(buildingID))
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", uint(buildingID)).Msg("查询入驻续约记录失败")
		utils.Error(c, http.StatusInternalServerError, "查询记录失败")
		return
	}
	utils.Success(c, gin.H{"records": records})
}

// RenewMy 公寓续约（当前房东管理员）：仅操作自己所属公寓
func (h *BuildingHandler) RenewMy(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req struct {
		ExpiredAt string `json:"expired_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	operator := c.GetString("username")
	if operator == "" {
		if uid, err := utils.GetUserID(c); err == nil {
			operator = fmt.Sprintf("uid:%d", uid)
		}
	}
	if err := h.BuildingService.Renew(bid, req.ExpiredAt, operator); err != nil {
		utils.Error(c, http.StatusBadRequest, err.Error())
		return
	}
	logger.Log.Info().Uint("building_id", bid).Str("expired_at", req.ExpiredAt).Msg("房东续约成功")
	utils.SuccessWithMsg(c, "续约成功，公寓已恢复展示", nil)
}

// RenewalsMy 获取当前房东管理员所属公寓的入驻/续约历史
func (h *BuildingHandler) RenewalsMy(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	records, err := h.BuildingService.ListRenewals(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询入驻续约记录失败")
		utils.Error(c, http.StatusInternalServerError, "查询记录失败")
		return
	}
	utils.Success(c, gin.H{"records": records})
}

// MyStats 获取当前管理员所属公寓的统计数据
func (h *BuildingHandler) MyStats(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	stats, err := h.BuildingService.GetWithStats(bid)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取公寓统计失败")
		utils.Error(c, http.StatusInternalServerError, "获取统计失败")
		return
	}
	utils.Success(c, gin.H{"stats": stats})
}

