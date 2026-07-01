package services

import (
	"rental-server/models"
	"rental-server/utils"

	"gorm.io/gorm"
)

type DividendService struct {
	DB *gorm.DB
}

func NewDividendService(db *gorm.DB) *DividendService {
	return &DividendService{DB: db}
}

func (s *DividendService) GetShareholders(buildingID uint) ([]models.Shareholder, error) {
	var shareholders []models.Shareholder
	err := s.DB.Where("building_id = ?", buildingID).Find(&shareholders).Error
	return shareholders, err
}

func (s *DividendService) CreateShareholder(shareholder *models.Shareholder) error {
	return s.DB.Create(shareholder).Error
}

func (s *DividendService) UpdateShareholder(id uint, updates map[string]interface{}) error {
	return s.DB.Model(&models.Shareholder{}).Where("id = ?", id).Updates(updates).Error
}

func (s *DividendService) DeleteShareholder(id uint) error {
	return s.DB.Delete(&models.Shareholder{}, id).Error
}

func (s *DividendService) Calculate(buildingID uint, month string) (map[string]interface{}, error) {
	var bills []models.Bill
	startDate := month + "-01"
	endDate := utils.Now().AddDate(0, 1, -1).Format("2006-01-02")

	err := s.DB.Where("building_id = ? AND bill_date >= ? AND bill_date <= ?", buildingID, startDate, endDate).
		Find(&bills).Error
	if err != nil {
		return nil, err
	}

	var totalIncome, totalExpense float64
	for _, b := range bills {
		if b.Type == "income" {
			totalIncome += b.Amount
		} else {
			totalExpense += b.Amount
		}
	}

	netProfit := totalIncome - totalExpense

	shareholders, err := s.GetShareholders(buildingID)
	if err != nil {
		return nil, err
	}

	var dividends []map[string]interface{}
	for _, sh := range shareholders {
		dividends = append(dividends, map[string]interface{}{
			"shareholder_id":   sh.ID,
			"shareholder_name": sh.Name,
			"share_ratio":      sh.ShareRatio,
			"dividend_amount":  netProfit * sh.ShareRatio / 100,
		})
	}

	return map[string]interface{}{
		"month":         month,
		"total_income":  totalIncome,
		"total_expense": totalExpense,
		"net_profit":    netProfit,
		"dividends":     dividends,
	}, nil
}

func (s *DividendService) List(buildingID uint) ([]models.Dividend, error) {
	var dividends []models.Dividend
	err := s.DB.Where("building_id = ?", buildingID).
		Preload("Shareholder").
		Order("settle_month DESC").
		Find(&dividends).Error
	return dividends, err
}

func (s *DividendService) Settle(dividend *models.Dividend) error {
	return s.DB.Create(dividend).Error
}

func (s *DividendService) Predict(buildingID uint, months int) ([]map[string]interface{}, error) {
	var recentBills []models.Bill
	err := s.DB.Where("building_id = ?", buildingID).
		Order("bill_date DESC").
		Limit(12).
		Find(&recentBills).Error
	if err != nil {
		return nil, err
	}

	var avgIncome, avgExpense float64
	if len(recentBills) > 0 {
		for _, b := range recentBills {
			if b.Type == "income" {
				avgIncome += b.Amount
			} else {
				avgExpense += b.Amount
			}
		}
		avgIncome /= float64(len(recentBills))
		avgExpense /= float64(len(recentBills))
	}

	var predictions []map[string]interface{}
	now := utils.Now()
	for i := 1; i <= months; i++ {
		t := now.AddDate(0, i, 0)
		month := t.Format("2006-01")

		predictions = append(predictions, map[string]interface{}{
			"month":     month,
			"rent":      avgIncome,
			"deposit":   0.0,
			"available": avgIncome - avgExpense,
		})
	}

	return predictions, nil
}
