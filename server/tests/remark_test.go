package tests

import (
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"rental-server/models"
	"rental-server/tests/testkit"
)

// remarkSecret 用来判断备注是否泄露到公开接口的唯一标记。
const remarkSecret = "内部备注_粤B12345_不可外泄"

// TestRemark_NotLeakedToPublic 备注只能出现在后台接口。
//
// 这是「备注字段」的安全红线测试：公寓/房东/房间/媒体四类备注会随公开接口
// （公寓列表/详情、房间列表/详情、位置选项）一起序列化，只要有一处漏了
// json:"-" 或忘了清洗，租客端就能看到内部信息（车牌号、收款账户等）。
func TestRemark_NotLeakedToPublic(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 造一个对公开端可见的公寓 + 空房 + 媒体，再写入各类备注
	b := models.Building{Name: "备注泄露检查楼", Status: "active",
		ExpiredAt: time.Now().AddDate(1, 0, 0).Format("2006-01-02"), Remark: remarkSecret}
	require.NoError(t, app.DB.Create(&b).Error)
	room := models.Room{BuildingID: b.ID, RoomNumber: "RK1", Floor: "1", Layout: "单间",
		Status: "vacant", Remark: remarkSecret}
	require.NoError(t, app.DB.Create(&room).Error)
	require.NoError(t, app.DB.Create(&models.BuildingLandlord{
		BuildingID: b.ID, Name: "房东甲", Phone: "13900000001", Remark: remarkSecret,
	}).Error)
	require.NoError(t, app.DB.Create(&models.RoomMedia{
		RoomID: room.ID, Type: "image", Category: "gallery",
		FilePath: "buildings/x/rooms/x/a.png", Status: "ready", Remark: remarkSecret,
	}).Error)

	publicPaths := []string{
		"/api/buildings",
		fmt.Sprintf("/api/buildings/%d", b.ID),
		fmt.Sprintf("/api/buildings/%d/rooms", b.ID),
		fmt.Sprintf("/api/buildings/%d/rooms/%d", b.ID, room.ID),
		"/api/locations/options",
	}
	for _, path := range publicPaths {
		r := app.Client.Do(http.MethodGet, path, "", nil)
		require.Equal(t, http.StatusOK, r.Status, "path=%s", path)
		body := string(r.Raw)
		require.NotContains(t, body, remarkSecret, "公开接口 %s 泄露了备注内容", path)
		require.NotContains(t, body, `"remark"`, "公开接口 %s 出现了 remark 字段", path)
	}

	// 后台接口必须能看到备注（否则功能等于没做）
	adminList := app.Client.Do(http.MethodGet, "/api/admin/buildings", f.TokenRoot, nil)
	adminList.RequireOK(t)
	require.Contains(t, string(adminList.Raw), remarkSecret, "后台公寓列表应包含备注")

	myBuilding := app.Client.Do(http.MethodGet, "/api/building/info", f.TokenA, nil)
	myBuilding.RequireOK(t)

	adminRooms := app.Client.Do(http.MethodGet, "/api/building/rooms", f.TokenA, nil)
	adminRooms.RequireOK(t)

	adminRoom := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil)
	if adminRoom.Status == http.StatusOK {
		require.Contains(t, string(adminRoom.Raw), remarkSecret, "后台房间详情应包含房间/媒体备注")
	}
}

// TestRemark_ContractFlow 合同备注：预订 → 确认签约继承 → 续租可改；退租原因拼进账单描述。
func TestRemark_ContractFlow(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	newRoom := models.Room{BuildingID: f.A.ID, RoomNumber: "RM1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&newRoom).Error)

	// 1) 交定金预订，带备注
	reserve := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", newRoom.ID), f.TokenA, map[string]any{
		"status": "reserved", "tenant_name": "王五", "tenant_phone": "13900000002",
		"rent_price": 1800, "management_fee": 180, "deposit": 1800, "earnest_money": 500,
		"start_date": "2027-01-01", "end_date": "2027-12-31",
		"remark": "有车，粤B12345；周末才回来住",
	})
	reserve.RequireOK(t)

	var contract models.RentalContract
	require.NoError(t, app.DB.Where("room_id = ?", newRoom.ID).First(&contract).Error)
	require.Equal(t, "有车，粤B12345；周末才回来住", contract.Remark, "预订备注应落库")

	// 2) 确认签约（续租接口先改备注，验证单条可修改）
	renew := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/contract", newRoom.ID), f.TokenA, map[string]any{
		"end_date": "2028-01-31", "remark": "已换车牌：粤B54321",
	})
	renew.RequireOK(t)
	require.NoError(t, app.DB.First(&contract, contract.ID).Error)
	require.Equal(t, "已换车牌：粤B54321", contract.Remark, "续租应能修改备注")

	// 3) 退租（不填原因也可提交；填了要拼进账单描述）
	needEnd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", newRoom.ID), f.TokenA, map[string]any{
		"status": "rented", "tenant_name": "王五", "tenant_phone": "13900000002",
		"rent_price": 1800, "management_fee": 180, "deposit": 1800,
		"start_date": "2026-01-01", "end_date": "2027-12-31", "remark": "已换车牌：粤B54321",
	})
	needEnd.RequireOK(t)

	vacate := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", newRoom.ID), f.TokenA, map[string]any{
		"status": "vacant", "refunded_deposit": 1500, "remark": "卫生扣200、墙面损坏扣100",
	})
	vacate.RequireOK(t)

	var bill models.Bill
	require.NoError(t, app.DB.Where("subtype = ?", "押金退还").Order("id DESC").First(&bill).Error)
	require.Contains(t, bill.Description, "卫生扣200、墙面损坏扣100", "退租原因应拼进账单描述")
	require.Equal(t, "卫生扣200、墙面损坏扣100", bill.Remark, "退租原因应同时写入账单内部备注")
}

// TestRemark_TenantBillDividendTask 租客/账单/股东/分红/待办备注的落库与回读。
func TestRemark_TenantBillDividendTask(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 租客备注（仅后台可改）
	tenantID := f.ContractA102.TenantID
	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/tenants/%d", tenantID), f.TokenA, map[string]any{
		"remark": "车牌粤B11111，有宠物", "emergency_contact": "母亲 13900000009",
	})
	r.RequireOK(t)
	var tenant models.Tenant
	require.NoError(t, app.DB.First(&tenant, tenantID).Error)
	require.Equal(t, "车牌粤B11111，有宠物", tenant.Remark)
	require.Equal(t, "母亲 13900000009", tenant.EmergencyContact)

	// 后台房间详情应能读到租客备注（ContractView → TenantView 的嵌套要透出）
	detail := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["A102"].ID), f.TokenA, nil)
	detail.RequireOK(t)
	require.Contains(t, string(detail.Raw), "车牌粤B11111，有宠物", "后台房间详情应带出租客备注")
	// 合同列表也应带出租客备注
	contracts := app.Client.Do(http.MethodGet, "/api/building/contracts", f.TokenA, nil)
	contracts.RequireOK(t)
	require.Contains(t, string(contracts.Raw), "车牌粤B11111，有宠物", "合同列表应带出租客备注")

	// 但公开接口绝不能出现（匿名访客拿不到合同，更拿不到备注）
	pubRoom := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/buildings/%d/rooms/%d", f.A.ID, f.Rooms["A102"].ID), "", nil)
	pubRoom.RequireOK(t)
	require.NotContains(t, string(pubRoom.Raw), "车牌粤B11111，有宠物")

	// 账单备注（与 description 分开）
	createBill := app.Client.Do(http.MethodPost, "/api/building/bills", f.TokenA, map[string]any{
		"type": "income", "subtype": "其他", "amount": 88, "bill_date": f.Today,
		"description": "其他收入", "remark": "楼下快递柜租金",
	})
	createBill.RequireOK(t) // 201
	var bill models.Bill
	require.NoError(t, app.DB.Where("remark = ?", "楼下快递柜租金").First(&bill).Error)
	require.Equal(t, "其他收入", bill.Description, "内部备注不能覆盖系统描述")

	// 股东备注
	sh := app.Client.Do(http.MethodPost, "/api/building/dividends/shareholders", f.TokenA, map[string]any{
		"name": "测试股东", "share_ratio": 10, "remark": "代持，联系方式13800000000",
	})
	sh.RequireOK(t)
	var shareholder models.Shareholder
	require.NoError(t, app.DB.Where("name = ?", "测试股东").First(&shareholder).Error)
	require.Equal(t, "代持，联系方式13800000000", shareholder.Remark)

	// 分红结算备注（写入当月每条分红）
	settle := app.Client.Do(http.MethodPost, "/api/building/dividends/settle", f.TokenA, map[string]any{
		"month": f.LastMonth, "remark": "本月含大额维修",
	})
	if settle.Status == http.StatusOK {
		var d models.Dividend
		require.NoError(t, app.DB.Where("settle_month = ?", f.LastMonth).First(&d).Error)
		require.Equal(t, "本月含大额维修", d.Remark)
	}

	// 待办处理备注
	task := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/building/tasks/%d/process", f.TaskA104.ID), f.TokenA, map[string]any{
		"refunded_deposit": 100, "remark": "墙面有钉孔，扣100",
	})
	task.RequireOK(t)
	var got models.Task
	require.NoError(t, app.DB.First(&got, f.TaskA104.ID).Error)
	require.Equal(t, "墙面有钉孔，扣100", got.ResultRemark)
}

// TestRemark_ImportRentedAndRecruit 批量导入的每行备注、招商申请备注。
func TestRemark_ImportRentedAndRecruit(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	imp := app.Client.Do(http.MethodPost, "/api/building/rooms/import-rented", f.TokenA, map[string]any{
		"rooms": []map[string]any{{
			"room_number": "IM1", "floor": "1", "layout": "单间",
			"rent_price": 1500, "deposit_months": 1, "management_fee": 150,
			"electricity_unit_price": 1.2, "water_unit_price": 4,
			"tenant_name": "导入租客", "tenant_phone": "13900000003",
			"deposit": 1500, "start_date": "2026-01-01", "end_date": "2027-01-01",
			"remark": "前房东欠3个月管理费，钥匙在门卫",
		}},
	})
	imp.RequireOK(t)
	var room models.Room
	require.NoError(t, app.DB.Where("room_number = ?", "IM1").First(&room).Error)
	var contract models.RentalContract
	require.NoError(t, app.DB.Where("room_id = ?", room.ID).First(&contract).Error)
	require.Equal(t, "前房东欠3个月管理费，钥匙在门卫", contract.Remark)

	// 招商申请备注：公开提交 → 后台列表可见
	sub := app.Client.Do(http.MethodPost, "/api/recruit/submit", "", map[string]any{
		"phone": "13900000004", "address": "南山区科技园X栋", "remark": "整栋12间，预算面谈",
	})
	sub.RequireOK(t)
	adminList := app.Client.Do(http.MethodGet, "/api/admin/recruit/list", f.TokenRoot, nil)
	adminList.RequireOK(t)
	require.Contains(t, string(adminList.Raw), "整栋12间，预算面谈")
}

// TestRemark_LocationCustom 位置备注：后台可见，公开选项不泄露。
func TestRemark_LocationCustom(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	add := app.Client.Do(http.MethodPost, "/api/admin/locations/custom", f.TokenRoot, map[string]any{
		"level": 1, "district": "测试区", "name": "测试区", "remark": remarkSecret,
	})
	add.RequireOK(t)

	public := app.Client.Do(http.MethodGet, "/api/locations/options", "", nil)
	public.RequireOK(t)
	require.NotContains(t, string(public.Raw), remarkSecret, "公开位置选项泄露了备注")
	require.False(t, strings.Contains(string(public.Raw), `"remark"`))

	manage := app.Client.Do(http.MethodGet, "/api/admin/locations/manage", f.TokenRoot, nil)
	manage.RequireOK(t)
	require.Contains(t, string(manage.Raw), remarkSecret, "后台位置管理应包含备注")
}

// TestRemark_BuildingSettingsInternal 楼栋管理员可在「公寓设置」维护公寓内部备注。
func TestRemark_BuildingSettingsInternal(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodPut, "/api/building/info", f.TokenA, map[string]any{
		"remark": "押金走私人账户",
	})
	r.RequireOK(t)
	info := app.Client.Do(http.MethodGet, "/api/building/info", f.TokenA, nil)
	info.RequireOK(t)
	require.Contains(t, string(info.Raw), "押金走私人账户")

	// 公开公寓详情仍然看不到
	pub := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/buildings/%d", f.A.ID), "", nil)
	pub.RequireOK(t)
	require.NotContains(t, string(pub.Raw), "押金走私人账户")
}

// TestRemark_MediaAndRoomEdit 房间内部备注、媒体说明的编辑与后台回读。
func TestRemark_MediaAndRoomEdit(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	room := models.Room{BuildingID: f.A.ID, RoomNumber: "RM2", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)

	upd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{
		"remark": "钥匙在门卫处，热水器需预热5分钟",
	})
	upd.RequireOK(t)
	get := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil)
	get.RequireOK(t)
	require.Contains(t, string(get.Raw), "钥匙在门卫处", "后台房间详情应能看到房间备注")

	media := models.RoomMedia{RoomID: room.ID, Type: "image", Category: "gallery",
		FilePath: "buildings/x/rooms/x/b.png", Status: "ready"}
	require.NoError(t, app.DB.Create(&media).Error)
	mr := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/media/%d", room.ID, media.ID), f.TokenA, map[string]any{
		"remark": "主卧带飘窗",
	})
	mr.RequireOK(t)
	get2 := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, nil)
	get2.RequireOK(t)
	require.Contains(t, string(get2.Raw), "主卧带飘窗", "后台房间详情应能看到媒体说明")

	// 清空备注（单条可修改）
	clear := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", room.ID), f.TokenA, map[string]any{
		"remark": "",
	})
	clear.RequireOK(t)
	var got models.Room
	require.NoError(t, app.DB.First(&got, room.ID).Error)
	require.Empty(t, got.Remark, "空串应清空备注")
}

// TestRemark_BuildingLandlordRenew 公寓/房东/续约备注。
func TestRemark_BuildingLandlordRenew(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	upd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, map[string]any{
		"remark": "押金走私人账户，月底对账",
		"landlords": []map[string]any{
			{"name": "房东甲", "phone": "13900000001", "remark": "只收现金"},
		},
	})
	upd.RequireOK(t)

	var b models.Building
	require.NoError(t, app.DB.First(&b, f.A.ID).Error)
	require.Equal(t, "押金走私人账户，月底对账", b.Remark)
	var l models.BuildingLandlord
	require.NoError(t, app.DB.Where("building_id = ?", f.A.ID).First(&l).Error)
	require.Equal(t, "只收现金", l.Remark)

	// 续约备注
	renew := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/buildings/%d/renew", f.A.ID), f.TokenRoot, map[string]any{
		"expired_at": time.Now().AddDate(2, 0, 0).Format("2006-01-02"),
		"remark":     "房东同意再签两年，押金不变",
	})
	renew.RequireOK(t)
	var r models.BuildingRenewal
	require.NoError(t, app.DB.Where("building_id = ? AND action = ?", f.A.ID, "renew").Order("id DESC").First(&r).Error)
	require.Equal(t, "房东同意再签两年，押金不变", r.Remark)

	// 房东备注不得出现在公开接口
	pub := app.Client.Do(http.MethodGet, fmt.Sprintf("/api/buildings/%d", f.A.ID), "", nil)
	pub.RequireOK(t)
	require.NotContains(t, string(pub.Raw), "只收现金")
	require.False(t, strings.Contains(string(pub.Raw), `"remark"`))
}

// TestRemark_ReserveRemarkInherited 预订备注在确认签约时必须延续：
// 调用方没传 remark 不能把预订时填的车牌号冲掉（此前 service 无条件覆盖）。
func TestRemark_ReserveRemarkInherited(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)
	room := models.Room{BuildingID: f.A.ID, RoomNumber: "RI1", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room).Error)

	reserve := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", room.ID), f.TokenA, map[string]any{
		"status": "reserved", "tenant_name": "王五", "tenant_phone": "13900000002",
		"rent_price": 1800, "management_fee": 180, "deposit": 1800, "earnest_money": 500,
		"start_date": "2027-01-01", "end_date": "2027-12-31", "remark": "有车，粤B12345",
	})
	reserve.RequireOK(t)

	// ① 确认签约不传 remark：预订备注必须保留
	signNoRemark := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", room.ID), f.TokenA, map[string]any{
		"status": "rented", "tenant_name": "王五", "tenant_phone": "13900000002",
		"rent_price": 1800, "management_fee": 180, "deposit": 1800,
		"start_date": "2027-01-01", "end_date": "2027-12-31",
	})
	signNoRemark.RequireOK(t)
	var ct models.RentalContract
	require.NoError(t, app.DB.Where("room_id = ?", room.ID).First(&ct).Error)
	require.Equal(t, "有车，粤B12345", ct.Remark, "确认签约未传备注时，预订备注应保留")

	// ② 确认签约显式传了 remark：以新值为准
	room2 := models.Room{BuildingID: f.A.ID, RoomNumber: "RI2", Floor: "1", Layout: "单间", Status: "vacant"}
	require.NoError(t, app.DB.Create(&room2).Error)
	reserve2 := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", room2.ID), f.TokenA, map[string]any{
		"status": "reserved", "tenant_name": "王五", "tenant_phone": "13900000002",
		"rent_price": 1800, "management_fee": 180, "deposit": 1800, "earnest_money": 500,
		"start_date": "2027-01-01", "end_date": "2027-12-31", "remark": "旧备注",
	})
	reserve2.RequireOK(t)
	signRemark := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d/status", room2.ID), f.TokenA, map[string]any{
		"status": "rented", "tenant_name": "王五", "tenant_phone": "13900000002",
		"rent_price": 1800, "management_fee": 180, "deposit": 1800,
		"start_date": "2027-01-01", "end_date": "2027-12-31", "remark": "已换车牌：粤B54321",
	})
	signRemark.RequireOK(t)
	// 用新变量接收：ct 里已有上一条合同的主键，直接 First(&ct) 会被 GORM 加上 id 条件
	var ct2 models.RentalContract
	require.NoError(t, app.DB.Where("room_id = ?", room2.ID).First(&ct2).Error)
	require.Equal(t, "已换车牌：粤B54321", ct2.Remark, "确认签约传了备注时应覆盖")
}

// TestRemark_BuildingAdminCanEditOwnAccount 楼栋管理员维护本公寓账号的姓名/电话/备注，
// 且不能改角色、不能跨公寓操作（超管接口 403）。
func TestRemark_BuildingAdminCanEditOwnAccount(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 楼栋管理员改自己公寓下的普通管理员 → 成功
	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/auth/users/%d", f.APlain.ID), f.TokenA, map[string]any{
		"display_name": "前台小张", "phone": "13900000009", "remark": "负责1楼，夜班",
	})
	r.RequireOK(t)
	var got models.User
	require.NoError(t, app.DB.First(&got, f.APlain.ID).Error)
	require.Equal(t, "前台小张", got.DisplayName)
	require.Equal(t, "13900000009", got.Phone)
	require.Equal(t, "负责1楼，夜班", got.Remark)
	require.Equal(t, "admin", got.Role, "楼栋管理员不得改角色")

	// 清空备注（单条可修改）
	clear := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/auth/users/%d", f.APlain.ID), f.TokenA, map[string]any{
		"remark": "",
	})
	clear.RequireOK(t)
	require.NoError(t, app.DB.First(&got, f.APlain.ID).Error)
	require.Empty(t, got.Remark)

	// 跨公寓 → 404（不能动 B 栋的账号）
	cross := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/auth/users/%d", f.BUser.ID), f.TokenA, map[string]any{
		"remark": "越权",
	})
	require.Equal(t, http.StatusNotFound, cross.Status)

	// 超管接口对楼栋管理员仍然是 403（两条路分开）
	adminRoute := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/auth/users/%d", f.APlain.ID), f.TokenA, map[string]any{
		"remark": "越权",
	})
	require.Equal(t, http.StatusForbidden, adminRoute.Status)

	// 楼栋管理员不能删除自己
	selfDel := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/auth/users/%d", f.AUser.ID), f.TokenA, nil)
	require.Equal(t, http.StatusBadRequest, selfDel.Status)

	// 楼栋管理员删除本公寓普通管理员 → 成功
	del := app.Client.Do(http.MethodDelete, fmt.Sprintf("/api/building/auth/users/%d", f.APlain.ID), f.TokenA, nil)
	del.RequireOK(t)
}

// TestRemark_ClipTo500 备注类字段统一截断到 500 字符（MySQL varchar(500) 不会报 Data too long）。
func TestRemark_ClipTo500(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	long := strings.Repeat("备", 600)
	r := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/building/rooms/%d", f.Rooms["A101"].ID), f.TokenA, map[string]any{
		"remark": long,
	})
	r.RequireOK(t)
	var room models.Room
	require.NoError(t, app.DB.First(&room, f.Rooms["A101"].ID).Error)
	require.Len(t, []rune(room.Remark), 500, "备注应被裁剪到 500 字符")
}

// TestRemark_LandlordRemarkPreservedOnEdit 编辑公寓时没动房东备注不能把原备注冲掉；
// 显式传空串才清空。
func TestRemark_LandlordRemarkPreservedOnEdit(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 初始写入两个房东行、各自不同备注
	upd := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, map[string]any{
		"landlords": []map[string]any{
			{"name": "房东甲", "phone": "13900000001", "remark": "只收现金"},
			{"name": "房东乙", "phone": "13900000007", "remark": "代持，找小王"},
		},
	})
	upd.RequireOK(t)

	// ① 不带 remark 字段再编辑一次：两个房东的备注都还在
	again := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, map[string]any{
		"landlords": []map[string]any{
			{"name": "房东甲", "phone": "13900000001"},
			{"name": "房东乙", "phone": "13900000007"},
		},
	})
	again.RequireOK(t)
	var ls []models.BuildingLandlord
	require.NoError(t, app.DB.Where("building_id = ?", f.A.ID).Order("id ASC").Find(&ls).Error)
	require.Len(t, ls, 2)
	remarks := map[string]string{}
	for _, l := range ls {
		remarks[l.Name] = l.Remark
	}
	require.Equal(t, "只收现金", remarks["房东甲"], "没动备注时不应被冲掉")
	require.Equal(t, "代持，找小王", remarks["房东乙"], "第二行的备注也不能被冲掉")

	// ② 显式传空串 → 清空
	clear := app.Client.Do(http.MethodPut, fmt.Sprintf("/api/admin/buildings/%d", f.A.ID), f.TokenRoot, map[string]any{
		"landlords": []map[string]any{
			{"name": "房东甲", "phone": "13900000001", "remark": ""},
		},
	})
	clear.RequireOK(t)
	require.NoError(t, app.DB.Where("building_id = ?", f.A.ID).Order("id ASC").Find(&ls).Error)
	require.Len(t, ls, 1)
	require.Empty(t, ls[0].Remark, "显式传空串应清空备注")
}

// TestRemark_PlatformTaskResult 平台级待办可写处理备注，且空 body 也能处理。
func TestRemark_PlatformTaskResult(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	// 造一条平台级待办
	task := models.Task{BuildingID: &f.D.ID, Title: "公寓即将到期", Type: "building_expiring",
		Priority: "high", Status: "pending", Scope: "platform", DueDate: f.D.ExpiredAt}
	require.NoError(t, app.DB.Create(&task).Error)

	// 空 body（旧客户端）→ 仍可处理，且不带备注
	empty := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/tasks/%d/process", task.ID), f.TokenRoot, nil)
	empty.RequireOK(t)

	// 带备注处理
	task2 := models.Task{BuildingID: &f.D.ID, Title: "公寓已到期", Type: "building_expired",
		Priority: "high", Status: "pending", Scope: "platform"}
	require.NoError(t, app.DB.Create(&task2).Error)
	withRemark := app.Client.Do(http.MethodPost, fmt.Sprintf("/api/admin/tasks/%d/process", task2.ID), f.TokenRoot, map[string]any{
		"remark": "已电话联系房东，等回复",
	})
	withRemark.RequireOK(t)
	var got models.Task
	require.NoError(t, app.DB.First(&got, task2.ID).Error)
	require.Equal(t, "completed", got.Status)
	require.Equal(t, "已电话联系房东，等回复", got.ResultRemark)
}
