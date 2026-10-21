package handlers

import (
	"testing"

	"rental-server/models"
	"rental-server/services"
)

// setupTaskHandler 造待办处理器。
func setupTaskHandler(t *testing.T, e *testEnv) *TaskHandler {
	t.Helper()
	return &TaskHandler{DB: e.db, TaskService: services.NewTaskService(e.db)}
}

// 退租待办处理：生成押金退还账单、结束合同、房间置空、待办完成。
func TestProcessExpiredRoomTask(t *testing.T) {
	e := newTestEnv(t)
	h := setupTaskHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 1, tenant.ID, 2000)
	task := models.Task{BuildingID: ptrU(1), Title: "房间到期退租", Type: "expired_room", Status: "pending", RoomID: ptrU(1)}
	if err := e.db.Create(&task).Error; err != nil {
		t.Fatalf("造待办失败: %v", err)
	}

	e.eng.POST("/tasks/:id/process", h.Process)
	w := e.do(t, "POST", "/tasks/"+itoa(task.ID)+"/process", 1, map[string]any{"refunded_deposit": 1800})
	expectOK(t, w)

	// 押金退还账单
	if n := countBills(t, e.db, "room_id = ? AND subtype = ?", 1, "押金退还"); n != 1 {
		t.Fatalf("应生成 1 条押金退还账单，实际 %d 条", n)
	}
	var bill models.Bill
	e.db.Where("room_id = ? AND subtype = ?", 1, "押金退还").First(&bill)
	if bill.Amount != 1800 || bill.Type != "expense" {
		t.Fatalf("退还账单应为支出 1800，实际 type=%s amount=%v", bill.Type, bill.Amount)
	}
	// 合同结束
	var ct models.RentalContract
	e.db.First(&ct, 1)
	if ct.Status != "ended" {
		t.Fatalf("合同状态应为 ended，实际 %q", ct.Status)
	}
	// 房间空置
	if got := getRoom(t, e.db, 1).Status; got != "vacant" {
		t.Fatalf("房间状态应为 vacant，实际 %q", got)
	}
	// 待办完成
	e.db.First(&task, task.ID)
	if task.Status != "completed" {
		t.Fatalf("待办状态应为 completed，实际 %q", task.Status)
	}
}

// 退租待办处理：退还 0 元不生成账单；负数金额被拒绝；已处理待办不能重复处理。
func TestProcessExpiredRoomTaskRules(t *testing.T) {
	e := newTestEnv(t)
	h := setupTaskHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedRoom(t, e.db, 1, 1, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 1, tenant.ID, 2000)
	task := models.Task{BuildingID: ptrU(1), Title: "房间到期退租", Type: "expired_room", Status: "pending", RoomID: ptrU(1)}
	if err := e.db.Create(&task).Error; err != nil {
		t.Fatalf("造待办失败: %v", err)
	}

	e.eng.POST("/tasks/:id/process", h.Process)

	// 负数金额
	w := e.do(t, "POST", "/tasks/"+itoa(task.ID)+"/process", 1, map[string]any{"refunded_deposit": -1})
	expectFail(t, w)

	// 未处理过，房间仍出租
	if got := getRoom(t, e.db, 1).Status; got != "rented" {
		t.Fatalf("失败的请求不应改变房间状态，实际 %q", got)
	}

	// 退还 0 元：处理成功但不生成账单
	w = e.do(t, "POST", "/tasks/"+itoa(task.ID)+"/process", 1, map[string]any{"refunded_deposit": 0})
	expectOK(t, w)
	if n := countBills(t, e.db, "subtype = ?", "押金退还"); n != 0 {
		t.Fatalf("退还 0 元不应生成账单，实际 %d 条", n)
	}

	// 已处理的待办不能重复处理
	w = e.do(t, "POST", "/tasks/"+itoa(task.ID)+"/process", 1, map[string]any{"refunded_deposit": 100})
	expectFail(t, w)
}

// 跨公寓不能处理他人待办。
func TestProcessTaskCrossBuildingRejected(t *testing.T) {
	e := newTestEnv(t)
	h := setupTaskHandler(t, e)
	seedBuilding(t, e.db, 1)
	seedBuilding(t, e.db, 2)
	seedRoom(t, e.db, 1, 2, "rented")
	tenant := seedTenant(t, e.db, "张三")
	seedActiveContract(t, e.db, 1, 1, 2, tenant.ID, 2000)
	task := models.Task{BuildingID: ptrU(2), Title: "房间到期退租", Type: "expired_room", Status: "pending", RoomID: ptrU(1)}
	if err := e.db.Create(&task).Error; err != nil {
		t.Fatalf("造待办失败: %v", err)
	}

	e.eng.POST("/tasks/:id/process", h.Process)
	w := e.do(t, "POST", "/tasks/"+itoa(task.ID)+"/process", 1, map[string]any{"refunded_deposit": 100})
	expectFail(t, w)
	if got := getRoom(t, e.db, 1).Status; got != "rented" {
		t.Fatalf("越权处理不应改变房间状态，实际 %q", got)
	}
}
