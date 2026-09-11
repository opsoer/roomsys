package handlers

import (
	"net/http"
	"strconv"
	"strings"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// LocationManageHandler 平台位置字典管理：
// 静态官方区划（前端内置，只读）之外的增改均落库；改名在事务内级联同步公寓表。
type LocationManageHandler struct {
	DB *gorm.DB
}

type locationReq struct {
	Level    int    `json:"level"`
	District string `json:"district"`
	Street   string `json:"street"`
	Name     string `json:"name"`
	OldName  string `json:"old_name"`
	NewName  string `json:"new_name"`
	CustomID uint   `json:"custom_id"`
	DryRun   bool   `json:"dry_run"`
}

// 归一化：同一条目带或不带「街道/社区」后缀视为同一地方（与公开端筛选口径一致）
func normalizeStreetSuffix(s string) string {
	return strings.TrimSuffix(s, "街道")
}

func normalizeVillageSuffix(s string) string {
	return strings.TrimSuffix(s, "社区")
}

func (h *LocationManageHandler) badRequest(c *gin.Context, msg string) {
	utils.Error(c, http.StatusBadRequest, msg)
}

// List 返回自定义条目与静态条目改名映射，供前端与静态基座合并出完整选项树
func (h *LocationManageHandler) List(c *gin.Context) {
	var customs []models.LocationCustom
	if err := h.DB.Order("id ASC").Find(&customs).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	var renames []models.LocationRename
	if err := h.DB.Order("id ASC").Find(&renames).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "查询失败")
		return
	}
	utils.Success(c, gin.H{"customs": customs, "renames": renames})
}

// AddCustom 新增自定义位置条目（区域/街道/村小区），自定义表内重复时拒绝
func (h *LocationManageHandler) AddCustom(c *gin.Context) {
	var req locationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "参数错误")
		return
	}
	req.District = strings.TrimSpace(req.District)
	req.Street = strings.TrimSpace(req.Street)
	req.Name = strings.TrimSpace(req.Name)
	switch req.Level {
	case 1:
		if req.District == "" {
			h.badRequest(c, "区域名称不能为空")
			return
		}
		req.Name = req.District
		req.Street = ""
	case 2:
		if req.District == "" || req.Name == "" {
			h.badRequest(c, "缺少所属区域或街道名称")
			return
		}
		req.Street = ""
	case 3:
		if req.District == "" || req.Street == "" || req.Name == "" {
			h.badRequest(c, "缺少所属区域、街道或村/小区名称")
			return
		}
	default:
		h.badRequest(c, "无效的级别")
		return
	}

	if h.customExists(req.Level, req.District, req.Street, req.Name, nil) {
		h.badRequest(c, "该自定义条目已存在")
		return
	}
	entry := models.LocationCustom{Level: req.Level, District: req.District, Street: req.Street, Name: req.Name}
	if err := h.DB.Create(&entry).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "创建失败")
		return
	}
	utils.Success(c, gin.H{"custom": entry})
}

// DeleteCustom 删除自定义条目：只要还有公寓（含软删除的）引用该位置就禁止，避免位置信息丢失
func (h *LocationManageHandler) DeleteCustom(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		h.badRequest(c, "无效的ID")
		return
	}
	var entry models.LocationCustom
	if err := h.DB.First(&entry, uint(id)).Error; err != nil {
		utils.Error(c, http.StatusNotFound, "条目不存在")
		return
	}
	cond, args := h.buildingCond(entry.Level, entry.District, entry.Street, entry.Name)
	var count int64
	h.DB.Model(&models.Building{}).Where(cond, args...).Count(&count)
	if count > 0 {
		h.badRequest(c, "该位置下仍有公寓在用，禁止删除")
		return
	}
	// 删除上级位置前，要求其下的自定义子条目已先删除，避免留下永不显示的孤儿条目
	var childCount int64
	switch entry.Level {
	case 1:
		h.DB.Model(&models.LocationCustom{}).Where("level = 2 AND district = ?", entry.Name).Count(&childCount)
		if childCount > 0 {
			h.badRequest(c, "该区域下仍有自定义街道，请先删除")
			return
		}
	case 2:
		h.DB.Model(&models.LocationCustom{}).Where("level = 3 AND district = ? AND street = ?", entry.District, entry.Name).Count(&childCount)
		if childCount > 0 {
			h.badRequest(c, "该街道下仍有自定义村/小区，请先删除")
			return
		}
	}
	if err := h.DB.Delete(&entry).Error; err != nil {
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	utils.Success(c, gin.H{"ok": true})
}

// Rename 改名：dry_run=true 只返回受影响公寓数供前端确认弹窗展示；
// 确认后在事务内级联更新公寓表。custom_id 非空表示改的是自定义条目（直接改自身记录），
// 否则视为对静态官方条目的改名，写入改名映射供前端把静态基座渲染成新名。
func (h *LocationManageHandler) Rename(c *gin.Context) {
	var req locationReq
	if err := c.ShouldBindJSON(&req); err != nil {
		h.badRequest(c, "参数错误")
		return
	}
	req.OldName = strings.TrimSpace(req.OldName)
	req.NewName = strings.TrimSpace(req.NewName)
	req.District = strings.TrimSpace(req.District)
	req.Street = strings.TrimSpace(req.Street)
	if req.OldName == "" || req.NewName == "" {
		h.badRequest(c, "原名称与新名称不能为空")
		return
	}
	if req.OldName == req.NewName {
		h.badRequest(c, "新名称与原名称相同")
		return
	}
	if req.Level < 1 || req.Level > 3 {
		h.badRequest(c, "无效的级别")
		return
	}
	if req.Level >= 2 && req.District == "" {
		h.badRequest(c, "缺少所属区域")
		return
	}
	if req.Level == 3 && req.Street == "" {
		h.badRequest(c, "缺少所属街道")
		return
	}

	buildingCond, buildingArgs := h.buildingCond(req.Level, req.District, req.Street, req.OldName)
	var buildingCount int64
	h.DB.Model(&models.Building{}).Where(buildingCond, buildingArgs...).Count(&buildingCount)

	if req.DryRun {
		utils.Success(c, gin.H{"affected_buildings": buildingCount})
		return
	}

	if h.customExists(req.Level, req.District, req.Street, req.NewName, nil) {
		h.badRequest(c, "新名称已存在同名自定义条目，请先处理")
		return
	}

	cascadeField := map[int]string{1: "district", 2: "street", 3: "village"}[req.Level]

	err := h.DB.Transaction(func(tx *gorm.DB) error {
		if req.CustomID > 0 {
			updates := map[string]interface{}{"name": req.NewName}
			if req.Level == 1 {
				updates["district"] = req.NewName // 区域条目的 district 与 name 相同，需一并更新
			}
			res := tx.Model(&models.LocationCustom{}).Where("id = ?", req.CustomID).Updates(updates)
			if res.Error != nil {
				return res.Error
			}
			if res.RowsAffected == 0 {
				// 自定义条目不存在（可能已被删除），退化为映射方式保证改名不丢
				if err := tx.Create(&models.LocationRename{
					Level: req.Level, District: req.District, Street: req.Street,
					OldName: req.OldName, NewName: req.NewName,
				}).Error; err != nil {
					return err
				}
			}
		} else {
			if err := tx.Create(&models.LocationRename{
				Level: req.Level, District: req.District, Street: req.Street,
				OldName: req.OldName, NewName: req.NewName,
			}).Error; err != nil {
				return err
			}
		}
		if buildingCount > 0 {
			if err := tx.Model(&models.Building{}).Where(buildingCond, buildingArgs...).Update(cascadeField, req.NewName).Error; err != nil {
				return err
			}
		}
		// 链式改名跟随：此前 a→b 的映射，在 b→c 之后也要指向 c
		renameCond, renameArgs := renameChainCond(req.Level, req.District, req.Street, req.OldName)
		if err := tx.Model(&models.LocationRename{}).Where(renameCond, renameArgs...).Update("new_name", req.NewName).Error; err != nil {
			return err
		}
		// 上级位置改名时，自定义条目与改名映射的上下文字段同步跟进，避免挂在旧上级下的条目失联
		switch req.Level {
		case 1:
			if err := tx.Model(&models.LocationCustom{}).Where("district = ?", req.OldName).Update("district", req.NewName).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.LocationRename{}).Where("district = ?", req.OldName).Update("district", req.NewName).Error; err != nil {
				return err
			}
		case 2:
			if err := tx.Model(&models.LocationCustom{}).Where("district = ? AND street = ?", req.District, req.OldName).Update("street", req.NewName).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.LocationRename{}).Where("district = ? AND street = ?", req.District, req.OldName).Update("street", req.NewName).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		logger.Log.Error().Err(err).Msg("位置改名失败")
		utils.Error(c, http.StatusInternalServerError, "改名失败")
		return
	}
	utils.Success(c, gin.H{"affected_buildings": buildingCount})
}

// buildingCond 构造公寓表按位置匹配（含后缀兼容）的条件
func (h *LocationManageHandler) buildingCond(level int, district, street, name string) (string, []interface{}) {
	switch level {
	case 1:
		return "district = ?", []interface{}{name}
	case 2:
		return "district = ? AND street IN (?, ?)", []interface{}{district, name, normalizeStreetSuffix(name)}
	default:
		return "district = ? AND street IN (?, ?) AND village IN (?, ?)",
			[]interface{}{district, street, normalizeStreetSuffix(street), name, normalizeVillageSuffix(name)}
	}
}

// customExists 校验自定义表内是否已有同位置条目
func (h *LocationManageHandler) customExists(level int, district, street, name string, excludeID *uint) bool {
	var count int64
	query := h.DB.Model(&models.LocationCustom{})
	switch level {
	case 1:
		query = query.Where("level = 1 AND name = ?", name)
	case 2:
		query = query.Where("level = 2 AND district = ? AND name = ?", district, name)
	case 3:
		query = query.Where("level = 3 AND district = ? AND street = ? AND name = ?", district, street, name)
	}
	if excludeID != nil {
		query = query.Where("id <> ?", *excludeID)
	}
	query.Count(&count)
	return count > 0
}

// renameChainCond 构造改名映射链式更新的条件（已有映射的新名等于本次旧名时跟随更新）
func renameChainCond(level int, district, street, oldName string) (string, []interface{}) {
	switch level {
	case 1:
		return "level = 1 AND new_name = ?", []interface{}{oldName}
	case 2:
		return "level = 2 AND district = ? AND new_name = ?", []interface{}{district, oldName}
	default:
		return "level = 3 AND district = ? AND street = ? AND new_name = ?", []interface{}{district, street, oldName}
	}
}
