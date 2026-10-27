package tests

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"rental-server/models"
	"rental-server/tests/testkit"
)

func countWhere(t *testing.T, db *gorm.DB, model any, query string, args ...any) int64 {
	t.Helper()
	var n int64
	require.NoError(t, db.Model(model).Where(query, args...).Count(&n).Error)
	return n
}

func getRoom(t *testing.T, app *testkit.App, id uint) models.Room {
	t.Helper()
	var r models.Room
	require.NoError(t, app.DB.First(&r, id).Error)
	return r
}

func getContract(t *testing.T, app *testkit.App, id uint) models.RentalContract {
	t.Helper()
	var c models.RentalContract
	require.NoError(t, app.DB.First(&c, id).Error)
	return c
}

func getBill(t *testing.T, app *testkit.App, id uint) models.Bill {
	t.Helper()
	var b models.Bill
	require.NoError(t, app.DB.First(&b, id).Error)
	return b
}

func getTask(t *testing.T, app *testkit.App, id uint) models.Task {
	t.Helper()
	var task models.Task
	require.NoError(t, app.DB.First(&task, id).Error)
	return task
}

func uptr(v uint) *uint { return &v }
