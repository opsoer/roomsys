package services

import (
	"fmt"
	"rental-server/models"
	"rental-server/utils"
	"time"

	"gorm.io/gorm"
)

type BillService struct {
	DB *gorm.DB
}

func NewBillService(db *gorm.DB) *BillService {
	return &BillService{DB: db}
}

func (s *BillService) GetByID(id uint) (*models.Bill, error) {
	var bill models.Bill
	err := s.DB.Preload("Room").First(&bill, id).Error
	return &bill, err
}

func (s *BillService) List(buildingID uint, params map[string]interface{}) ([]models.Bill, error) {
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

	err := query.Preload("Room").Order("bill_date DESC, id DESC").Find(&bills).Error
	return bills, err
}

func (s *BillService) Create(bill *models.Bill) error {
	return s.DB.Create(bill).Error
}

func (s *BillService) Update(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Bill{}).Where("id = ?", id).Updates(updates).Error
}

func (s *BillService) Delete(id uint) error {
	return s.DB.Delete(&models.Bill{}, id).Error
}

func (s *BillService) GetStats(buildingID uint, month, year string) (map[string]interface{}, error) {
	result := map[string]interface{}{
		"total_income":  0.0,
		"total_expense": 0.0,
		"net_profit":    0.0,
		"income_detail": []map[string]interface{}{},
		"expense_detail": []map[string]interface{}{},
	}

	var bills []models.Bill
	query := s.DB.Where("building_id = ?", buildingID)

	if month != "" {
		startDate := month + "-01"
		t, _ := time.Parse("2006-01", month)
		endDate := t.AddDate(0, 1, -1).Format("2006-01-02")
		query = query.Where("bill_date >= ? AND bill_date <= ?", startDate, endDate)
	} else if year != "" {
		query = query.Where("bill_date >= ? AND bill_date <= ?", year+"-01-01", year+"-12-31")
	}

	if err := query.Find(&bills).Error; err != nil {
		return nil, err
	}

	incomeBySubtype := make(map[string]float64)
	expenseBySubtype := make(map[string]float64)

	for _, b := range bills {
		if b.Type == "income" {
			result["total_income"] = result["total_income"].(float64) + b.Amount
			incomeBySubtype[b.Subtype] += b.Amount
		} else {
			result["total_expense"] = result["total_expense"].(float64) + b.Amount
			expenseBySubtype[b.Subtype] += b.Amount
		}
	}

	result["net_profit"] = result["total_income"].(float64) - result["total_expense"].(float64)

	var incomeDetail []map[string]interface{}
	for st, total := range incomeBySubtype {
		incomeDetail = append(incomeDetail, map[string]interface{}{"subtype": st, "total": total})
	}
	result["income_detail"] = incomeDetail

	var expenseDetail []map[string]interface{}
	for st, total := range expenseBySubtype {
		expenseDetail = append(expenseDetail, map[string]interface{}{"subtype": st, "total": total})
	}
	result["expense_detail"] = expenseDetail

	return result, nil
}

func (s *BillService) GetTrend(buildingID uint, years int) (map[string]interface{}, error) {
	now := utils.Now()
	var months []map[string]interface{}
	var growth []map[string]interface{}

	for i := years - 1; i >= 0; i-- {
		t := now.AddDate(0, -i, 0)
		month := t.Format("2006-01")

		var incomeTotal, expenseTotal float64
		startDate := month + "-01"
		endDate := t.AddDate(0, 1, -1).Format("2006-01-02")

		s.DB.Model(&models.Bill{}).
			Where("building_id = ? AND type = 'income' AND bill_date >= ? AND bill_date <= ?", buildingID, startDate, endDate).
			Select("COALESCE(SUM(amount), 0)").Scan(&incomeTotal)

		s.DB.Model(&models.Bill{}).
			Where("building_id = ? AND type = 'expense' AND bill_date >= ? AND bill_date <= ?", buildingID, startDate, endDate).
			Select("COALESCE(SUM(amount), 0)").Scan(&expenseTotal)

		months = append(months, map[string]interface{}{
			"month":   month,
			"income":  incomeTotal,
			"expense": expenseTotal,
			"profit":  incomeTotal - expenseTotal,
		})
	}

	for i := 1; i < len(months); i++ {
		curr := months[i]
		prev := months[i-1]

		incomeMom := 0.0
		expenseMom := 0.0
		if prev["income"].(float64) > 0 {
			incomeMom = (curr["income"].(float64) - prev["income"].(float64)) / prev["income"].(float64) * 100
		}
		if prev["expense"].(float64) > 0 {
			expenseMom = (curr["expense"].(float64) - prev["expense"].(float64)) / prev["expense"].(float64) * 100
		}

		growth = append(growth, map[string]interface{}{
			"month":       curr["month"],
			"income_mom":  incomeMom,
			"expense_mom": expenseMom,
		})
	}

	return map[string]interface{}{
		"months": months,
		"growth": growth,
	}, nil
}

func (s *BillService) GenerateBillNo(buildingID uint) (string, error) {
	now := utils.Now()
	datePart := now.Format("20060102")

	var count int64
	s.DB.Model(&models.Bill{}).
		Where("building_id = ? AND bill_no LIKE ?", buildingID, fmt.Sprintf("B%s%%", datePart)).
		Count(&count)

	return fmt.Sprintf("B%s%05d", datePart, count+1), nil
}
