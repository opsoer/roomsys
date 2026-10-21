package config

import "testing"

func TestSchedulerDefaults(t *testing.T) {
	cfg := defaults()
	normalize(cfg)

	if cfg.Scheduler.RentBills.Time != "03:00" || !cfg.Scheduler.RentBills.Enabled {
		t.Fatalf("rent_bills 默认值不符: %+v", cfg.Scheduler.RentBills)
	}
	if cfg.Scheduler.TokenCleanup.IntervalHours != 12 {
		t.Fatalf("token_cleanup 默认值不符: %+v", cfg.Scheduler.TokenCleanup)
	}
	if cfg.Scheduler.DBBackup.Time != "04:00" || cfg.Scheduler.DBBackup.Keep != 7 {
		t.Fatalf("db_backup 默认值不符: %+v", cfg.Scheduler.DBBackup)
	}
}

func TestNormalizeJobFillsDefaults(t *testing.T) {
	cfg := defaults()
	cfg.Scheduler.RentBills = JobConfig{Enabled: true}
	cfg.Scheduler.TokenCleanup = JobConfig{Enabled: true}
	cfg.Scheduler.DBBackup = JobConfig{Enabled: true}
	normalize(cfg)

	if cfg.Scheduler.RentBills.Time != "03:00" {
		t.Fatalf("缺少周期应回退默认时刻: %+v", cfg.Scheduler.RentBills)
	}
	if cfg.Scheduler.TokenCleanup.IntervalHours != 12 || cfg.Scheduler.TokenCleanup.Time != "" {
		t.Fatalf("interval 任务应回退默认间隔: %+v", cfg.Scheduler.TokenCleanup)
	}
	if cfg.Scheduler.DBBackup.Keep != 7 {
		t.Fatalf("keep 应回退默认份数: %+v", cfg.Scheduler.DBBackup)
	}
}

func TestNormalizeJobNormalizesTime(t *testing.T) {
	cfg := defaults()
	cfg.Scheduler.RentBills = JobConfig{Enabled: true, Time: "3:5"}
	normalize(cfg)

	if cfg.Scheduler.RentBills.Time != "03:05" {
		t.Fatalf("时间未规范化: %q", cfg.Scheduler.RentBills.Time)
	}
}

func TestNormalizeJobTimeWinsOverInterval(t *testing.T) {
	cfg := defaults()
	cfg.Scheduler.TokenCleanup = JobConfig{Enabled: true, Time: "05:30", IntervalHours: 6}
	normalize(cfg)

	if cfg.Scheduler.TokenCleanup.Time != "05:30" || cfg.Scheduler.TokenCleanup.IntervalHours != 0 {
		t.Fatalf("time 应优先并清空间隔: %+v", cfg.Scheduler.TokenCleanup)
	}
}

func TestNormalizeJobInvalidTimePanics(t *testing.T) {
	cfg := defaults()
	cfg.Scheduler.RentBills = JobConfig{Enabled: true, Time: "25:00"}
	defer func() {
		if recover() == nil {
			t.Fatal("非法时间应 panic 以便尽早暴露配置错误")
		}
	}()
	normalize(cfg)
}

func TestNormalizeJobRejectsTrailingGarbage(t *testing.T) {
	cfg := defaults()
	cfg.Scheduler.RentBills = JobConfig{Enabled: true, Time: "03:00:00"}
	defer func() {
		if recover() == nil {
			t.Fatal("多余内容应 panic（避免静默接受错误配置）")
		}
	}()
	normalize(cfg)
}
