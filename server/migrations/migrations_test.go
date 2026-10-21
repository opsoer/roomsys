package migrations

import (
	"testing"

	"rental-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// newTestDB 建一个内存数据库（迁移台账表由迁移过程自身创建）。
func newTestDB(t *testing.T) *gorm.DB {
	t.Helper()
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatalf("打开内存数据库失败: %v", err)
	}
	if sqlDB, err := db.DB(); err == nil {
		t.Cleanup(func() { sqlDB.Close() })
	}
	return db
}

// 迁移可在空库上完整执行，且台账记录了版本。
func TestApplyOnEmptyDatabase(t *testing.T) {
	db := newTestDB(t)

	executed, err := Apply(db)
	if err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	if len(executed) != 1 || executed[0] != 1 {
		t.Fatalf("应执行 1 个版本，实际 %v", executed)
	}

	var records []models.SchemaMigration
	if err := db.Find(&records).Error; err != nil {
		t.Fatalf("读取台账失败: %v", err)
	}
	if len(records) != 1 || records[0].Version != 1 || records[0].Name != "init_schema" {
		t.Fatalf("台账记录不符: %+v", records)
	}
}

// 重复执行是幂等的：不会重复建表、不会重复记账。
func TestApplyIsIdempotent(t *testing.T) {
	db := newTestDB(t)

	if _, err := Apply(db); err != nil {
		t.Fatalf("首次迁移失败: %v", err)
	}
	executed, err := Apply(db)
	if err != nil {
		t.Fatalf("重复迁移失败: %v", err)
	}
	if len(executed) != 0 {
		t.Fatalf("重复执行不应有新版本，实际 %v", executed)
	}
	var count int64
	db.Model(&models.SchemaMigration{}).Count(&count)
	if count != 1 {
		t.Fatalf("台账应只有 1 条记录，实际 %d 条", count)
	}
}

// 状态查询：已应用与待执行分开返回。
func TestStatus(t *testing.T) {
	db := newTestDB(t)

	applied, pending, err := Status(db)
	if err != nil {
		t.Fatalf("查询状态失败: %v", err)
	}
	if len(applied) != 0 || len(pending) != 1 {
		t.Fatalf("初始状态不符: applied=%v pending=%d", applied, len(pending))
	}

	if _, err := Apply(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}
	applied, pending, err = Status(db)
	if err != nil {
		t.Fatalf("查询状态失败: %v", err)
	}
	if len(applied) != 1 || len(pending) != 0 {
		t.Fatalf("迁移后状态不符: applied=%v pending=%d", applied, len(pending))
	}
}

// 漂移检测：迁移后的表结构必须覆盖模型定义的每一个字段。
// 若有人改了 models 却忘了追加编号步骤，这个测试会失败，从而拦住「代码要字段、库里没列」的事故。
func TestMigratedSchemaMatchesModels(t *testing.T) {
	db := newTestDB(t)
	if _, err := Apply(db); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	for _, m := range models.AllModels() {
		stmt := &gorm.Statement{DB: db}
		if err := stmt.Parse(m); err != nil {
			t.Fatalf("解析模型失败: %v", err)
		}
		table := stmt.Schema.Table
		if !db.Migrator().HasTable(m) {
			t.Errorf("模型 %T 对应的表 %s 不存在", m, table)
			continue
		}
		for _, field := range stmt.Schema.Fields {
			if field.DBName == "" {
				continue
			}
			if !db.Migrator().HasColumn(m, field.DBName) {
				t.Errorf("表 %s 缺少字段 %s（模型 %T 定义了它，请补充 migrations 编号步骤）",
					table, field.DBName, m)
			}
		}
	}
}
