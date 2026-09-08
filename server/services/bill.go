// 账单服务，提供账单的增删改查和统计分析
package services

import (
	"rental-server/models"
	"rental-server/utils"
	"time"

	"gorm.io/gorm"
)

// BillService 账单服务
type BillService struct {
	DB *gorm.DB
}

// NewBillService 创建账单服务实例
func NewBillService(db *gorm.DB) *BillService {
	return &BillService{DB: db}
}

// GetByID 根据ID获取账单
func (s *BillService) GetByID(id uint) (*models.Bill, error) {
	var bill models.Bill
	err := s.DB.Preload("Room", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).First(&bill, id).Error
	return &bill, err
}

// List 分页查询账单列表（支持按类型、日期、房间号筛选）
// 双模式分页：lastID > 0 时走游标分页（按 bill_date + id 定位下一批），不再重查总数（total 返回 -1）
func (s *BillService) List(buildingID uint, params map[string]interface{}, page, lastID, size int, lastKey string) ([]models.Bill, int64, error) {
	var bills []models.Bill
	query := s.DB.Where("building_id = ?", buildingID)

	if t, ok := params["type"]; ok && t != "" {
		query = query.Where("type = ?", t)
	}
	if st, ok := params["subtype"]; ok && st != "" {
		query = query.Where("subtype = ?", st)
	}
	if sd, ok := params["start_date"]; ok && sd != "" {
		query = query.Where("bill_date >= ?", sd)
	}
	if ed, ok := params["end_date"]; ok && ed != "" {
		query = query.Where("bill_date <= ?", ed)
	}
	if rn, ok := params["room_number"]; ok && rn != "" {
		if rn == "other" {
			query = query.Where("room_id IS NULL")
		} else {
			query = query.Where("room_id IN (?)", s.DB.Table("rooms").Select("id").Where("room_number = ?", rn))
		}
	}

	if lastID > 0 {
		query = query.Where("(bill_date < ? OR (bill_date = ? AND id < ?))", lastKey, lastKey, lastID)
	}

	var total int64
	if lastID == 0 {
		if err := query.Model(&models.Bill{}).Count(&total).Error; err != nil {
			return nil, 0, err
		}
	} else {
		total = -1
	}

	q := query.Preload("Room", func(db *gorm.DB) *gorm.DB { return db.Unscoped() }).Order("bill_date DESC, id DESC")
	if lastID > 0 {
		err := q.Limit(size).Find(&bills).Error
		return bills, total, err
	}
	err := q.Offset((page - 1) * size).Limit(size).Find(&bills).Error
	return bills, total, err
}

// Create 创建账单
func (s *BillService) Create(bill *models.Bill) error {
	return s.DB.Create(bill).Error
}

// Update 更新账单信息
func (s *BillService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Bill{}).Where("id = ?", id).Updates(updates).Error
}

// Delete 删除账单
func (s *BillService) Delete(id uint) error {
	return s.DB.Delete(&models.Bill{}, id).Error
}

// GetStats 统计指定楼栋的收入/支出汇总（按月或年）。
// 用 SQL GROUP BY 一次成型，替代全量载入内存聚合；明细按金额降序，
// 避免此前 map 遍历导致饼图每次刷新顺序、配色随机变化。
func (s *BillService) GetStats(buildingID uint, month, year string) (map[string]interface{}, error) {
	type subtypeSum struct {
		Type    string
		Subtype string
		Cnt     int64
		Total   float64
	}

	query := s.DB.Model(&models.Bill{}).
		Select("type, COALESCE(subtype, '') as subtype, COUNT(*) as cnt, COALESCE(SUM(amount), 0) as total").
		Where("building_id = ?", buildingID)

	if month != "" {
		t, err := time.Parse("2006-01", month)
		if err != nil {
			return nil, err
		}
		query = query.Where("bill_date >= ? AND bill_date <= ?", month+"-01", t.AddDate(0, 1, -1).Format("2006-01-02"))
	} else if year != "" {
		query = query.Where("bill_date >= ? AND bill_date <= ?", year+"-01-01", year+"-12-31")
	}

	var sums []subtypeSum
	if err := query.Group("type, subtype").Order("total DESC").Scan(&sums).Error; err != nil {
		return nil, err
	}

	var totalIncome, totalExpense float64
	var billCount int64
	incomeDetail := []map[string]interface{}{}
	expenseDetail := []map[string]interface{}{}
	for _, r := range sums {
		billCount += r.Cnt
		item := map[string]interface{}{"subtype": r.Subtype, "total": r.Total, "count": r.Cnt}
		if r.Type == "income" {
			totalIncome += r.Total
			incomeDetail = append(incomeDetail, item)
		} else {
			totalExpense += r.Total
			expenseDetail = append(expenseDetail, item)
		}
	}

	return map[string]interface{}{
		"total_income":   totalIncome,
		"total_expense":  totalExpense,
		"net_profit":     totalIncome - totalExpense,
		"bill_count":     billCount,
		"income_detail":  incomeDetail,
		"expense_detail": expenseDetail,
	}, nil
}

// GetTrend 获取月度收支趋势及环比/同比数据。
// 月份按自然月补零填充为连续序列，避免无账单月份断档导致趋势线跳月、
// 环比跨月失真；同比需上一年同月数据，因此查询窗口向前多取 12 个月。
func (s *BillService) GetTrend(buildingID uint, years int) (map[string]interface{}, error) {
	if years <= 0 {
		years = 12
	}
	now := utils.Now()
	// 展示窗口：近 years 个自然月（含当月）
	displayStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location()).AddDate(0, -(years - 1), 0)
	// 查询窗口：展示窗口起点再往前推 12 个月，用于计算同比
	queryStart := displayStart.AddDate(-1, 0, 0)

	type monthlySum struct {
		Month  string
		Type   string
		Amount float64
	}
	var sums []monthlySum
	// bill_date 为 'YYYY-MM-DD' 字符串，SUBSTR 截前 7 位在 MySQL/SQLite 下语义一致
	err := s.DB.Model(&models.Bill{}).
		Select("SUBSTR(bill_date, 1, 7) as month, type, COALESCE(SUM(amount), 0) as amount").
		Where("building_id = ? AND bill_date >= ? AND bill_date <= ?", buildingID, queryStart.Format("2006-01-01"), now.Format("2006-12-31")).
		Group("month, type").
		Scan(&sums).Error
	if err != nil {
		return nil, err
	}

	amountBy := make(map[string]float64, len(sums)*2) // "月份|类型" -> 金额
	for _, r := range sums {
		amountBy[r.Month+"|"+r.Type] = r.Amount
	}

	// 从查询窗口起点到当月逐月补零，生成完整连续序列
	type monthRow struct {
		Month   string
		Income  float64
		Expense float64
	}
	var all []monthRow
	for t := queryStart; !t.After(now); t = t.AddDate(0, 1, 0) {
		m := t.Format("2006-01")
		all = append(all, monthRow{
			Month:   m,
			Income:  amountBy[m+"|income"],
			Expense: amountBy[m+"|expense"],
		})
	}

	// 仅输出展示窗口内的月份；此时起始下标 ≥ 12，环比/同比均可取到前置值
	var resultMonths []map[string]interface{}
	var growth []map[string]interface{}
	for i := len(all) - years; i < len(all); i++ {
		r := all[i]
		resultMonths = append(resultMonths, map[string]interface{}{
			"month":   r.Month,
			"income":  r.Income,
			"expense": r.Expense,
			"profit":  r.Income - r.Expense,
		})

		// 上期值 ≤ 0 时增长率无意义，返回 null（前端展示 '-'）
		pct := func(curr, prev float64) interface{} {
			if prev <= 0 {
				return nil
			}
			return (curr - prev) / prev * 100
		}
		prev := all[i-1]
		prevYear := all[i-12]
		growth = append(growth, map[string]interface{}{
			"month":       r.Month,
			"income_mom":  pct(r.Income, prev.Income),
			"expense_mom": pct(r.Expense, prev.Expense),
			"income_yoy":  pct(r.Income, prevYear.Income),
			"expense_yoy": pct(r.Expense, prevYear.Expense),
		})
	}

	return map[string]interface{}{
		"months": resultMonths,
		"growth": growth,
	}, nil
}

// GenerateBillNo 生成唯一账单编号（按日全局递增，避免跨楼栋同日重复；统计含软删除行防止编号冲突）
func (s *BillService) GenerateBillNo(buildingID uint) (string, error) {
	datePart := utils.Now().Format("20060102")
	return utils.NextBillNo(s.DB, datePart), nil
}
