package handlers

import (
	"encoding/csv"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"rental-server/logger"
	"rental-server/models"
	"rental-server/services"
	"rental-server/utils"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

func isMonthSettled(db *gorm.DB, buildingID uint, billDate string) bool {
	if len(billDate) < 7 {
		return false
	}
	month := billDate[:7]
	var count int64
	db.Model(&models.Dividend{}).
		Where("building_id = ? AND settle_month = ?", buildingID, month).
		Count(&count)
	return count > 0
}

func appendModification(desc, modLog string, maxMods int) string {
	const sep = " | "
	parts := strings.SplitN(desc, sep, maxMods+1)
	if len(parts) > maxMods {
		parts = parts[:maxMods]
	}
	base := parts[0]
	for i := 1; i < len(parts); i++ {
		base += sep + parts[i]
	}
	return base + sep + modLog
}

type BillHandler struct {
	DB           *gorm.DB
	BillService  *services.BillService
	RoomService  *services.RoomService
}

type CreateBillReq struct {
	Type        string  `json:"type" binding:"required"`
	Subtype     string  `json:"subtype" binding:"required"`
	Amount      float64 `json:"amount" binding:"required,gte=0"`
	RoomID      *uint   `json:"room_id"`
	Description string  `json:"description"`
	BillDate    string  `json:"bill_date" binding:"required"`
}

type UpdateBillReq struct {
	Type         string   `json:"type"`
	Subtype      string   `json:"subtype"`
	Amount       *float64 `json:"amount"`
	RoomID       *uint    `json:"room_id"`
	Description  string   `json:"description"`
	BillDate     string   `json:"bill_date"`
	ModifyReason string   `json:"modify_reason"`
}

func (h *BillHandler) List(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}

	params := map[string]interface{}{
		"type":        c.Query("type"),
		"subtype":     c.Query("subtype"),
		"room_id":     c.Query("room_id"),
		"room_number": c.Query("room_number"),
		"start_date":  c.Query("start_date"),
		"end_date":    c.Query("end_date"),
	}

	page, size := utils.ParsePage(c)
	bills, total, err := h.BillService.List(bid, params, page, size)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("查询账单列表失败")
		utils.Error(c, http.StatusInternalServerError, "查询账单列表失败")
		return
	}
	utils.Success(c, gin.H{"bills": bills, "total": total, "page": page, "size": size})
}

func (h *BillHandler) Create(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	var req CreateBillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("building_id", bid).Msg("创建账单请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.Type != "income" && req.Type != "expense" {
		logger.Log.Warn().Uint("building_id", bid).Str("type", req.Type).Msg("创建账单失败: 类型无效")
		utils.Error(c, http.StatusBadRequest, "类型必须为 income 或 expense")
		return
	}
	if isMonthSettled(h.DB, bid, req.BillDate) {
		logger.Log.Warn().Uint("building_id", bid).Str("bill_date", req.BillDate).Msg("创建账单失败: 该月已结算")
		utils.Error(c, http.StatusBadRequest, "该月已结算分红，无法创建账单")
		return
	}
	if req.Subtype == "其他" && req.Description == "" {
		logger.Log.Warn().Uint("building_id", bid).Msg("创建账单失败: 其他类型需备注")
		utils.Error(c, http.StatusBadRequest, "子类型为其他时，备注不能为空")
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
	billNo, err := h.BillService.GenerateBillNo(bid)
	if err != nil {
		logger.Log.Error().Err(err).Msg("生成账单编号失败")
		utils.Error(c, http.StatusInternalServerError, "创建账单失败")
		return
	}
	bill := models.Bill{
		BillNo:      billNo,
		Type:        req.Type,
		Subtype:     req.Subtype,
		Amount:      req.Amount,
		BuildingID:  bid,
		RoomID:      req.RoomID,
		Description: req.Description,
		BillDate:    req.BillDate,
		CreatedBy:   uid,
	}
	if err := h.BillService.Create(&bill); err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("创建账单数据库失败")
		utils.Error(c, http.StatusInternalServerError, "创建账单失败")
		return
	}
	logger.Log.Info().
		Uint("bill_id", bill.ID).
		Str("bill_no", billNo).
		Str("type", req.Type).
		Str("subtype", req.Subtype).
		Float64("amount", req.Amount).
		Uint("building_id", bid).
		Msg("账单创建成功")
	utils.Created(c, "创建成功", gin.H{"bill": bill})
}

func (h *BillHandler) Update(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	billID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的账单ID")
		return
	}
	bill, err := h.BillService.GetByID(uint(billID))
	if err != nil || bill.BuildingID != bid {
		logger.Log.Warn().Str("id", id).Uint("building_id", bid).Msg("更新账单失败: 账单不存在")
		utils.Error(c, http.StatusNotFound, "账单不存在")
		return
	}
	if isMonthSettled(h.DB, bid, bill.BillDate) {
		logger.Log.Warn().Uint("bill_id", bill.ID).Msg("更新账单失败: 该月已结算")
		utils.Error(c, http.StatusBadRequest, "该月已结算分红，无法修改账单")
		return
	}
	var req UpdateBillReq
	if err := c.ShouldBindJSON(&req); err != nil {
		logger.Log.Warn().Uint("bill_id", bill.ID).Msg("更新账单请求参数错误")
		utils.Error(c, http.StatusBadRequest, "参数错误")
		return
	}
	if req.ModifyReason == "" {
		logger.Log.Warn().Uint("bill_id", bill.ID).Msg("更新账单失败: 缺少修改原因")
		utils.Error(c, http.StatusBadRequest, "请填写修改原因")
		return
	}
	oldAmount := bill.Amount
	newAmount := bill.Amount
	if req.Amount != nil {
		newAmount = *req.Amount
	}
	updates := map[string]interface{}{
		"amount": newAmount,
	}
	modLog := fmt.Sprintf("[%s] 修改原因: %s", utils.Now().Format("2006-01-02 15:04:05"), req.ModifyReason)
	if req.Amount != nil {
		modLog += fmt.Sprintf(", 金额: %.2f -> %.2f", oldAmount, newAmount)
	}
	const maxMods = 10
	updates["description"] = appendModification(bill.Description, modLog, maxMods)
	if err := h.BillService.Update(bill.ID, updates); err != nil {
		logger.Log.Error().Err(err).Uint("bill_id", bill.ID).Msg("更新账单失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	updatedBill, err := h.BillService.GetByID(bill.ID)
	if err != nil {
		logger.Log.Error().Err(err).Uint("bill_id", bill.ID).Msg("获取更新后的账单失败")
		utils.Error(c, http.StatusInternalServerError, "更新失败")
		return
	}
	logger.Log.Info().
		Uint("bill_id", bill.ID).
		Str("bill_no", bill.BillNo).
		Float64("old_amount", oldAmount).
		Float64("new_amount", newAmount).
		Str("modify_reason", req.ModifyReason).
		Msg("账单更新成功")
	utils.Success(c, gin.H{"bill": updatedBill})
}

func (h *BillHandler) Delete(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	id := c.Param("id")
	billID, err := strconv.ParseUint(id, 10, 32)
	if err != nil {
		utils.Error(c, http.StatusBadRequest, "无效的账单ID")
		return
	}
	bill, err := h.BillService.GetByID(uint(billID))
	if err != nil || bill.BuildingID != bid {
		utils.Error(c, http.StatusNotFound, "账单不存在")
		return
	}
	if err := h.BillService.Delete(bill.ID); err != nil {
		logger.Log.Error().Err(err).Uint("bill_id", bill.ID).Msg("删除账单失败")
		utils.Error(c, http.StatusInternalServerError, "删除失败")
		return
	}
	utils.SuccessWithMsg(c, "删除成功", nil)
}

func (h *BillHandler) Stats(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}
	month := c.Query("month")
	year := c.Query("year")
	stats, err := h.BillService.GetStats(bid, month, year)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取统计数据失败")
		utils.Error(c, http.StatusInternalServerError, "获取统计失败")
		return
	}
	utils.Success(c, stats)
}

func (h *BillHandler) Trend(c *gin.Context) {
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
	years := 12
	if y := c.Query("years"); y != "" {
		if parsed, err := strconv.Atoi(y); err == nil {
			years = parsed
		}
	}
	trend, err := h.BillService.GetTrend(bid, years)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("获取趋势数据失败")
		utils.Error(c, http.StatusInternalServerError, "获取趋势失败")
		return
	}
	utils.Success(c, trend)
}

func (h *BillHandler) ExportCSV(c *gin.Context) {
	bid, err := utils.GetBuildingID(c)
	if err != nil {
		utils.Error(c, http.StatusUnauthorized, "未授权")
		return
	}

	params := map[string]interface{}{
		"type":       c.Query("type"),
		"subtype":    c.Query("subtype"),
		"start_date": c.Query("start_date"),
		"end_date":   c.Query("end_date"),
	}

	bills, _, err := h.BillService.List(bid, params, 1, 100000)
	if err != nil {
		logger.Log.Error().Err(err).Uint("building_id", bid).Msg("导出账单失败")
		utils.Error(c, http.StatusInternalServerError, "导出失败")
		return
	}

	c.Header("Content-Type", "text/csv; charset=utf-8")
	c.Header("Content-Disposition", "attachment; filename=bills.csv")

	c.Writer.Write([]byte("\xEF\xBB\xBF"))

	writer := csv.NewWriter(c.Writer)
	defer writer.Flush()

	writer.Write([]string{"账单编号", "日期", "类型", "子类型", "金额", "关联房间", "备注"})
	for _, b := range bills {
		room := ""
		if b.Room.RoomNumber != "" {
			room = b.Room.RoomNumber
		}
		writer.Write([]string{
			b.BillNo,
			b.BillDate,
			b.Type,
			b.Subtype,
			fmt.Sprintf("%.2f", b.Amount),
			room,
			b.Description,
		})
	}
}
