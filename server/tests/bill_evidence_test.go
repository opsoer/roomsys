// 账单凭证截图：上传、随单创建、事后补充、删除与孤儿文件 GC 的接口级回归。
package tests

import (
	"encoding/base64"
	"net/http"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/scheduler"
	"rental-server/tests/testkit"
)

// 1x1 PNG（合法图片魔数，过内容嗅探）
var evidenceTestPNG, _ = base64Std.DecodeString("iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==")

var base64Std = base64.StdEncoding

func uploadBillEvidence(t *testing.T, app *testkit.App, token, filename string) (string, string) {
	t.Helper()
	r := app.Client.Upload("/api/building/bills/evidences/upload", token, "file", filename, evidenceTestPNG, nil)
	r.RequireOK(t)
	var data struct {
		FileKey  string `json:"file_key"`
		FileName string `json:"file_name"`
	}
	r.Decode(t, &data)
	require.NotEmpty(t, data.FileKey)
	return data.FileKey, data.FileName
}

func TestBillEvidence_UploadCreateList(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	key, name := uploadBillEvidence(t, app, f.TokenA, "转账截图.png")
	require.Contains(t, key, "/bills/")

	// 随单创建：凭证直接挂上
	r := app.Client.Do(http.MethodPost, "/api/building/bills", f.TokenA, map[string]any{
		"type": "expense", "subtype": "维修", "amount": 300, "bill_date": f.Today,
		"description": "维修支出", "evidences": []map[string]any{
			{"file_key": key, "file_name": name, "file_size": len(evidenceTestPNG)},
		},
	})
	r.RequireOK(t)
	var created struct {
		Bill models.Bill `json:"bill"`
	}
	r.Decode(t, &created)
	require.Len(t, created.Bill.Evidences, 1)
	require.Equal(t, key, created.Bill.Evidences[0].FileKey)

	// 列表预加载凭证
	r2 := app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenA, nil)
	r2.RequireOK(t)
	var list struct {
		Bills []models.Bill `json:"bills"`
	}
	r2.Decode(t, &list)
	var found *models.Bill
	for i := range list.Bills {
		if list.Bills[i].ID == created.Bill.ID {
			found = &list.Bills[i]
		}
	}
	require.NotNil(t, found, "创建的账单应出现在列表中")
	require.Len(t, found.Evidences, 1)

	// 文件已落盘（本地存储模式）
	localPath := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(key))
	_, err := os.Stat(localPath)
	require.NoError(t, err, "凭证文件应存在于本地存储")
}

func TestBillEvidence_RejectForeignBuildingKey(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 用 B 公寓账号上传，拿 B 的 key
	keyB, _ := uploadBillEvidence(t, app, f.TokenB, "b.png")

	// A 公寓管理员试图把 B 的 key 挂到 A 的账单上 → 拒绝
	r := app.Client.Do(http.MethodPost, "/api/building/bills", f.TokenA, map[string]any{
		"type": "expense", "subtype": "维修", "amount": 100, "bill_date": f.Today,
		"evidences": []map[string]any{{"file_key": keyB, "file_name": "b.png"}},
	})
	require.Equal(t, http.StatusBadRequest, r.Status)

	// 伪造路径前缀 → 拒绝
	r2 := app.Client.Do(http.MethodPost, "/api/building/bills", f.TokenA, map[string]any{
		"type": "expense", "subtype": "维修", "amount": 100, "bill_date": f.Today,
		"evidences": []map[string]any{{"file_key": "buildings/999_other/bills/x.png"}},
	})
	require.Equal(t, http.StatusBadRequest, r2.Status)
}

func TestBillEvidence_SupplementAndDelete(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	bill := createBill(t, app, f.TokenA, "expense", "维修", 300, f.Today, "支出", nil)
	require.Empty(t, bill.Evidences)

	key1, _ := uploadBillEvidence(t, app, f.TokenA, "a.png")
	key2, _ := uploadBillEvidence(t, app, f.TokenA, "b.png")

	// 事后补充两张
	supURL := "/api/building/bills/" + itoa(bill.ID) + "/evidences"
	rs := app.Client.Do(http.MethodPut, supURL, f.TokenA, map[string]any{
		"evidences": []map[string]any{
			{"file_key": key1, "file_name": "a.png"},
			{"file_key": key2, "file_name": "b.png"},
		},
	})
	rs.RequireOK(t)
	var sup struct {
		Bill models.Bill `json:"bill"`
	}
	rs.Decode(t, &sup)
	require.Len(t, sup.Bill.Evidences, 2)

	// 删除其中一张
	delURL := "/api/building/bills/" + itoa(bill.ID) + "/evidences/" + itoa(sup.Bill.Evidences[0].ID)
	rd := app.Client.Do(http.MethodDelete, delURL, f.TokenA, nil)
	rd.RequireOK(t)

	var after struct {
		Bill models.Bill `json:"bill"`
	}
	var list struct {
		Bills []models.Bill `json:"bills"`
	}
	rq := app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenA, nil)
	rq.RequireOK(t)
	rq.Decode(t, &list)
	for _, b := range list.Bills {
		if b.ID == bill.ID {
			after.Bill = b
		}
	}
	require.Len(t, after.Bill.Evidences, 1)
	require.Equal(t, key2, after.Bill.Evidences[0].FileKey)

	// 编辑账单整单替换凭证：传空列表 → 清空
	ru := app.Client.Do(http.MethodPut, "/api/building/bills/"+itoa(bill.ID), f.TokenA, map[string]any{
		"amount": 300, "modify_reason": "替换凭证测试", "evidences": []map[string]any{},
	})
	ru.RequireOK(t)

	var list2 struct {
		Bills []models.Bill `json:"bills"`
	}
	rq2 := app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenA, nil)
	rq2.RequireOK(t)
	rq2.Decode(t, &list2)
	for _, b := range list2.Bills {
		if b.ID == bill.ID {
			require.Empty(t, b.Evidences, "整单替换为空后凭证应清空")
		}
	}
}

func TestBillEvidence_VacateAttachesToRefundBill(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// A102 有生效合同：办理退租并随单上传凭证 → 押金退还账单应带上凭证
	key, _ := uploadBillEvidence(t, app, f.TokenA, "退款凭证.png")
	room := f.Rooms["A102"]
	r := app.Client.Do(http.MethodPut, "/api/building/rooms/"+itoa(room.ID)+"/status", f.TokenA, map[string]any{
		"status": "vacant", "refunded_deposit": 1500.0,
		"evidences": []map[string]any{{"file_key": key, "file_name": "退款凭证.png"}},
	})
	r.RequireOK(t)

	var list struct {
		Bills []models.Bill `json:"bills"`
	}
	rl := app.Client.Do(http.MethodGet, "/api/building/bills", f.TokenA, nil)
	rl.RequireOK(t)
	rl.Decode(t, &list)
	var refund *models.Bill
	for i := range list.Bills {
		if list.Bills[i].Subtype == "押金退还" && list.Bills[i].RoomID != nil && *list.Bills[i].RoomID == room.ID {
			refund = &list.Bills[i]
		}
	}
	require.NotNil(t, refund, "退租应生成押金退还账单")
	require.Len(t, refund.Evidences, 1)
	require.Equal(t, key, refund.Evidences[0].FileKey)
}

func TestBillEvidence_GCCleansOrphans(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 已挂账的文件：GC 不得删
	attached, _ := uploadBillEvidence(t, app, f.TokenA, "keep.png")
	bill := createBill(t, app, f.TokenA, "expense", "维修", 300, f.Today, "支出", nil)
	rs := app.Client.Do(http.MethodPut, "/api/building/bills/"+itoa(bill.ID)+"/evidences", f.TokenA, map[string]any{
		"evidences": []map[string]any{{"file_key": attached, "file_name": "keep.png"}},
	})
	rs.RequireOK(t)

	// 孤儿文件：直接在本地存储里放一个超过宽限期的未引用文件
	orphanRel := "buildings/" + itoa(f.A.ID) + "_x/bills/orphan.png"
	orphanAbs := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(orphanRel))
	require.NoError(t, os.MkdirAll(filepath.Dir(orphanAbs), 0755))
	require.NoError(t, os.WriteFile(orphanAbs, evidenceTestPNG, 0644))
	stale := time.Now().Add(-scheduler.BillEvidenceGCGrace - time.Hour)
	require.NoError(t, os.Chtimes(orphanAbs, stale, stale))

	summary := scheduler.CleanupBillEvidenceFiles(app.DB, app.Cfg)
	require.Contains(t, summary, "清理1个孤儿")

	_, err := os.Stat(orphanAbs)
	require.True(t, os.IsNotExist(err), "孤儿文件应被清理")
	_, err = os.Stat(filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(attached)))
	require.NoError(t, err, "已挂账文件不应被清理")

	// 宽限期内的孤儿不删
	freshRel := "buildings/" + itoa(f.A.ID) + "_x/bills/fresh.png"
	freshAbs := filepath.Join(app.Cfg.Server.UploadDir, filepath.FromSlash(freshRel))
	require.NoError(t, os.WriteFile(freshAbs, evidenceTestPNG, 0644))
	scheduler.CleanupBillEvidenceFiles(app.DB, app.Cfg)
	_, err = os.Stat(freshAbs)
	require.NoError(t, err, "宽限期内的文件不应被清理")
}
