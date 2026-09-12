// 分红服务，提供股东管理、分红计算与结算
package services

import (
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

// DividendService 分红服务
type DividendService struct {
	DB *gorm.DB
}

// NewDividendService 创建分红服务实例
func NewDividendService(db *gorm.DB) *DividendService {
	return &DividendService{DB: db}
}

// GetShareholders 获取楼栋股东列表
func (s *DividendService) GetShareholders(buildingID uint) ([]models.Shareholder, error) {
	var shareholders []models.Shareholder
	err := s.DB.Where("building_id = ?", buildingID).Find(&shareholders).Error
	return shareholders, err
}

// CreateShareholder 创建股东
func (s *DividendService) CreateShareholder(shareholder *models.Shareholder) error {
	return s.DB.Create(shareholder).Error
}

// UpdateShareholder 更新股东信息
func (s *DividendService) UpdateShareholder(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Shareholder{}).Where("id = ?", id).Updates(updates).Error
}

// DeleteShareholder 删除股东
func (s *DividendService) DeleteShareholder(id uint) error {
	return s.DB.Delete(&models.Shareholder{}, id).Error
}

// List 分页查询分红记录
// 双模式分页：lastID > 0 时走游标分页（按 settle_month + id 定位下一批），不再重查总数（total 返回 -1）
func (s *DividendService) List(buildingID uint, page, lastID, size int, lastKey string) ([]models.Dividend, int64, error) {
	var dividends []models.Dividend
	query := s.DB.Where("building_id = ?", buildingID)
	if lastID > 0 {
		query = query.Where("(settle_month < ? OR (settle_month = ? AND id < ?))", lastKey, lastKey, lastID)
	}
	var total int64
	if lastID == 0 {
		if err := query.Model(&models.Dividend{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
	} else {
		total = -1
	}
	q := query.Preload("Shareholder").Order("settle_month DESC, id DESC")
	if lastID > 0 {
		err := q.Limit(size).Find(&dividends).Error
		return dividends, total, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Find(&dividends).Error
	return dividends, total, err
}

// Predict 基于当前生效租约预测未来数月的应收租金：
// 预计租金收入 = 所有生效合同的月租金+管理费合计；
// 可分配净利润 = 预计租金收入 - 近3个月月均支出。
func (s *DividendService) Predict(buildingID uint, months int) ([]map[string]interface{}, error) {
	var contracts []models.RentalContract
	if err := s.DB.Where("building_id = ? AND status = ?", buildingID, "active").Find(&contracts).Error; err != nil {
		return nil, err
	}
	var monthlyRent float64
	for _, ct := range contracts {
		monthlyRent += ct.RentPrice + ct.ManagementFee
	}

	// 近3个月月均支出（不含押金退还前先按全部支出统计，保守估计）
	expenseStart := utils.Now().AddDate(0, -3, 0).Format("2006-01-02")
	var expenseBills []models.Bill
	if err := s.DB.Where("building_id = ? AND type = ? AND bill_date >= ?", buildingID, "expense", expenseStart).
		Find(&expenseBills).Error; err != nil {
		return nil, err
	}
	expenseByMonth := make(map[string]float64)
	for _, b := range expenseBills {
		if len(b.BillDate) >= 7 {
			expenseByMonth[b.BillDate[:7]] += b.Amount
		}
	}
	var expenseSum float64
	for _, v := range expenseByMonth {
		expenseSum += v
	}
	avgExpense := expenseSum / 3.0

	var predictions []map[string]interface{}
	now := utils.Now()
	for i := 1; i <= months; i++ {
		t := now.AddDate(0, i, 0)
		month := t.Format("2006-01")

		predictions = append(predictions, map[string]interface{}{
			"month":     month,
			"rent":      monthlyRent,
			"deposit":   0.0,
			"available": monthlyRent - avgExpense,
		})
	}

	return predictions, nil
}
