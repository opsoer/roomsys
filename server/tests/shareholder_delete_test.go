package tests

import (
	"fmt"
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

// TestShareholderDelete_CascadesDividends 验证删股东时同事务软删其分红，
// 避免分红行继续指向已删股东。
func TestShareholderDelete_CascadesDividends(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	sh := models.Shareholder{BuildingID: f.A.ID, Name: "待删股东", ShareRatio: 10}
	require.NoError(t, app.DB.Create(&sh).Error)
	div := models.Dividend{
		BuildingID: f.A.ID, SettleMonth: "2099-12", ShareholderID: sh.ID,
		TotalIncome: 100, NetProfit: 100, DividendAmount: 10,
	}
	require.NoError(t, app.DB.Create(&div).Error)

	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/dividends/shareholders/%d", sh.ID), f.TokenA, nil)
	del.RequireOK(t)

	require.Zero(t, countWhere(t, app.DB, &models.Shareholder{}, "id = ?", sh.ID))
	require.Zero(t, countWhere(t, app.DB, &models.Dividend{}, "shareholder_id = ?", sh.ID))
}
