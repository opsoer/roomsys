package tests

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/database"
	"rental-server/migrations"
	"rental-server/models"
	"rental-server/scheduler"
	"rental-server/tests/testkit"
)

func TestData_Migrations(t *testing.T) {
	db, _ := testkit.NewDB(t)

	applied, err := migrations.Apply(db)
	require.NoError(t, err)
	require.Contains(t, applied, 1)

	again, err := migrations.Apply(db)
	require.NoError(t, err)
	require.Empty(t, again, "重复执行迁移应为空")

	done, pending, err := migrations.Status(db)
	require.NoError(t, err)
	require.Contains(t, done, 1)
	require.Empty(t, pending)

	var version models.SchemaMigration
	require.NoError(t, db.First(&version, 1).Error)
	require.NotEmpty(t, version.Name)
}

func TestData_SchemaMatchesModels(t *testing.T) {
	db, _ := testkit.NewDB(t)
	_, err := migrations.Apply(db)
	require.NoError(t, err)

	all := models.AllModels()
	require.Greater(t, len(all), 15)
	for _, model := range all {
		require.True(t, db.Migrator().HasTable(model), "missing table for %T", model)
	}
}

func TestData_SQLitePathCreatesDirectory(t *testing.T) {
	dir := t.TempDir()
	cfg := testkit.ConfigForDir(dir)
	nested := filepath.Join(dir, "a", "b", "rental.db")
	cfg.Database.SQLite.Path = nested
	cfg.Server.UploadDir = filepath.Join(dir, "media")
	cfg.Server.WebDistDir = filepath.Join(dir, "webdist")
	cfg.Server.LogDir = filepath.Join(dir, "logs")

	db, err := database.Open(cfg)
	require.NoError(t, err)
	sqlDB, err := db.DB()
	require.NoError(t, err)
	require.NoError(t, sqlDB.Close())

	_, err = os.Stat(nested)
	require.NoError(t, err)
}

func TestData_BackupWithoutQiniu(t *testing.T) {
	db, cfg := testkit.NewDB(t)
	_, err := migrations.Apply(db)
	require.NoError(t, err)

	result := scheduler.BackupDatabase(db, cfg)
	require.Contains(t, result, "备份失败")
	require.Contains(t, result, "七牛")
}

func TestData_CleanupSoftDeleted(t *testing.T) {
	db, _ := testkit.NewDB(t)
	_, err := migrations.Apply(db)
	require.NoError(t, err)

	// 直接复用 scheduler 的清理逻辑：90 天前软删除 + 最近软删除
	result := scheduler.AutoCleanupData(db)
	require.Contains(t, result, "数据清理")
	require.Contains(t, result, "完成")
}
