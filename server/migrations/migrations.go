// Package migrations 提供版本化的数据库结构迁移。
//
// 与「启动时 AutoMigrate 自动猜结构」的区别：
//   - 每次结构变更有唯一编号（001、002…），严格按编号顺序执行；
//   - 已执行的版本记在 schema_migrations 表里，变更历史可审计；
//   - 字段改名 / 类型变更 / 数据回填等「有数据影响」的变更必须显式写在步骤里，
//     不会像 AutoMigrate 那样只加一列新列、把历史数据留在旧列；
//   - 每个步骤可提供 Down，用于回退。
//
// 用法：
//   - 启动时：migrations.Apply(db)（只执行尚未执行的版本）；
//   - 运维脚本：go run ./scripts/migrate（同样逻辑 + 打印状态）；
//   - 新增变更：在 steps.go 里追加一个更高编号的 Step，并把逻辑放进独立文件。
package migrations

import (
	"fmt"
	"sort"
	"time"

	"rental-server/logger"
	"rental-server/models"

	"gorm.io/gorm"
)

// Step 一个编号迁移步骤。
type Step struct {
	// Version 版本号，从 1 开始递增，不可复用、不可修改。
	Version int
	// Name 步骤名（用于台账与日志展示）。
	Name string
	// Up 执行迁移。
	Up func(db *gorm.DB) error
	// Down 回退迁移（可选；未提供时该版本不可回退）。
	Down func(db *gorm.DB) error
}

// Label 返回步骤的展示名。
func (s Step) Label() string {
	return fmt.Sprintf("%03d_%s", s.Version, s.Name)
}

// Steps 返回全部迁移步骤（按版本号升序）。
func Steps() []Step {
	steps := []Step{
		{Version: 1, Name: "init_schema", Up: upInitSchema},
	}
	// 后续变更在此追加：{Version: 2, Name: "xxx", Up: upXxx, Down: downXxx}
	sort.Slice(steps, func(i, j int) bool { return steps[i].Version < steps[j].Version })
	return steps
}

// Apply 执行所有尚未应用的迁移，返回本次应用的版本号列表。
func Apply(db *gorm.DB) ([]int, error) {
	if err := ensureMigrationTable(db); err != nil {
		return nil, err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[int]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	var executed []int
	for _, s := range Steps() {
		if appliedSet[s.Version] {
			continue
		}
		// 每个步骤独立事务：失败只影响本步，已完成的版本保持已记账
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := s.Up(tx); err != nil {
				return err
			}
			return tx.Create(&models.SchemaMigration{
				Version:   s.Version,
				Name:      s.Name,
				AppliedAt: time.Now(),
			}).Error
		})
		if err != nil {
			return executed, fmt.Errorf("迁移 %s 失败: %w", s.Label(), err)
		}
		logger.Log.Info().Str("version", s.Label()).Msg("数据库迁移已应用")
		executed = append(executed, s.Version)
	}
	return executed, nil
}

// Rollback 回退到目标版本（exclusive）：回退所有版本号大于 target 的步骤。
func Rollback(db *gorm.DB, target int) ([]int, error) {
	if err := ensureMigrationTable(db); err != nil {
		return nil, err
	}
	applied, err := appliedVersions(db)
	if err != nil {
		return nil, err
	}
	appliedSet := make(map[int]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}

	steps := Steps()
	var rolled []int
	// 从大版本往小版本回退
	for i := len(steps) - 1; i >= 0; i-- {
		s := steps[i]
		if s.Version <= target || !appliedSet[s.Version] {
			continue
		}
		if s.Down == nil {
			return rolled, fmt.Errorf("迁移 %s 未提供回退实现，无法回退", s.Label())
		}
		err := db.Transaction(func(tx *gorm.DB) error {
			if err := s.Down(tx); err != nil {
				return err
			}
			return tx.Where("version = ?", s.Version).Delete(&models.SchemaMigration{}).Error
		})
		if err != nil {
			return rolled, fmt.Errorf("回退迁移 %s 失败: %w", s.Label(), err)
		}
		logger.Log.Info().Str("version", s.Label()).Msg("数据库迁移已回退")
		rolled = append(rolled, s.Version)
	}
	return rolled, nil
}

// Status 返回已应用版本与待执行步骤。
func Status(db *gorm.DB) (applied []int, pending []Step, err error) {
	if err := ensureMigrationTable(db); err != nil {
		return nil, nil, err
	}
	applied, err = appliedVersions(db)
	if err != nil {
		return nil, nil, err
	}
	appliedSet := make(map[int]bool, len(applied))
	for _, v := range applied {
		appliedSet[v] = true
	}
	for _, s := range Steps() {
		if !appliedSet[s.Version] {
			pending = append(pending, s)
		}
	}
	return applied, pending, nil
}

// ensureMigrationTable 确保迁移台账表存在。
func ensureMigrationTable(db *gorm.DB) error {
	return db.AutoMigrate(&models.SchemaMigration{})
}

// appliedVersions 读取已应用的版本号列表。
func appliedVersions(db *gorm.DB) ([]int, error) {
	var records []models.SchemaMigration
	if err := db.Order("version ASC").Find(&records).Error; err != nil {
		return nil, err
	}
	versions := make([]int, 0, len(records))
	for _, r := range records {
		versions = append(versions, r.Version)
	}
	return versions, nil
}
