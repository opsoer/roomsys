package scheduler

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"rental-server/config"
	"rental-server/models"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

// touch 创建文件并把修改时间拨到指定时刻（构造孤儿/宽限期内文件用）。
func touch(t *testing.T, path string, modTime time.Time) {
	t.Helper()
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatalf("创建 %s 失败: %v", path, err)
	}
	if err := os.Chtimes(path, modTime, modTime); err != nil {
		t.Fatalf("设置 %s 修改时间失败: %v", path, err)
	}
}

func TestGCLocal(t *testing.T) {
	dir := t.TempDir()
	billDir := filepath.Join(dir, "buildings", "7_demo", "bills")
	otherDir := filepath.Join(dir, "buildings", "8_demo", "media") // 非 bills 目录不扫描
	noBillsDir := filepath.Join(dir, "buildings", "9_demo")        // 无 bills 子目录属正常
	for _, d := range []string{billDir, otherDir, noBillsDir} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	old := time.Now().Add(-BillEvidenceGCGrace - time.Hour)
	referenced := map[string]bool{
		"buildings/7_demo/bills/kept.png": true,
	}
	touch(t, filepath.Join(billDir, "kept.png"), old)      // 被引用：过期也不删
	touch(t, filepath.Join(billDir, "orphan.png"), old)    // 孤儿且超宽限期：删
	touch(t, filepath.Join(billDir, "fresh.png"), time.Now()) // 孤儿但在宽限期内：留
	touch(t, filepath.Join(otherDir, "media.png"), old)    // 非 bills 目录：不扫描
	touch(t, filepath.Join(noBillsDir, "note.txt"), old)   // 楼栋根下散文件：不在 bills 目录，跳过
	if err := os.Mkdir(filepath.Join(billDir, "sub"), 0o755); err != nil {
		t.Fatal(err)
	} // bills 下子目录：跳过

	deleted, checked, err := gcLocal(dir, referenced, time.Now().Add(-BillEvidenceGCGrace))
	if err != nil {
		t.Fatalf("gcLocal 出错: %v", err)
	}
	if deleted != 1 || checked != 3 {
		t.Fatalf("期望删1扫3，实际删%d扫%d", deleted, checked)
	}
	if _, err := os.Stat(filepath.Join(billDir, "orphan.png")); !os.IsNotExist(err) {
		t.Fatalf("孤儿文件应被删除")
	}
	for _, kept := range []string{"kept.png", "fresh.png"} {
		if _, err := os.Stat(filepath.Join(billDir, kept)); err != nil {
			t.Fatalf("%s 应保留: %v", kept, err)
		}
	}

	// 根目录不存在时静默返回
	if d, c, err := gcLocal(filepath.Join(dir, "missing"), nil, time.Now()); err != nil || d != 0 || c != 0 {
		t.Fatalf("缺失根目录应 (0,0,nil)，实际 (%d,%d,%v)", d, c, err)
	}
}

// CleanupBillEvidenceFiles 本地存储分支：引用行来自 bill_evidences，其余走 gcLocal 同一套语义。
func TestCleanupBillEvidenceFilesLocal(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	if err := db.AutoMigrate(&models.BillEvidence{}); err != nil {
		t.Fatal(err)
	}
	if err := db.Create(&models.BillEvidence{
		BillID: 1, BuildingID: 7, FileKey: "buildings/7_demo/bills/kept.png", FileName: "kept.png",
	}).Error; err != nil {
		t.Fatal(err)
	}

	dir := t.TempDir()
	billDir := filepath.Join(dir, "buildings", "7_demo", "bills")
	if err := os.MkdirAll(billDir, 0o755); err != nil {
		t.Fatal(err)
	}
	old := time.Now().Add(-BillEvidenceGCGrace - time.Hour)
	touch(t, filepath.Join(billDir, "kept.png"), old)
	touch(t, filepath.Join(billDir, "orphan.png"), old)

	cfg := &config.Config{}
	cfg.Server.UploadDir = dir // Qiniu AccessKey 为空 → 走本地分支
	summary := CleanupBillEvidenceFiles(db, cfg)

	if !strings.Contains(summary, "扫描2个文件") || !strings.Contains(summary, "清理1个孤儿") {
		t.Fatalf("摘要不符: %s", summary)
	}
	if _, err := os.Stat(filepath.Join(billDir, "orphan.png")); !os.IsNotExist(err) {
		t.Fatalf("孤儿文件应被删除")
	}
	if _, err := os.Stat(filepath.Join(billDir, "kept.png")); err != nil {
		t.Fatalf("被引用文件应保留: %v", err)
	}
}
