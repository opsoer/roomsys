package tests

import (
	"net/http"
	"testing"

	"github.com/stretchr/testify/require"

	"rental-server/tests/testkit"
)

// TestRentControlEndpoint 验证租控表聚合接口：
//   - 未出租房间归入 vacant（图片上的空房表，无备注）；
//   - 已出租但合同即将到期（10 天内）的房间归入 expiring，备注写到期时间；
//   - 已预订、租期尚远的房间不导出；
//   - 数据按公寓隔离；未登录返回 401。
func TestRentControlEndpoint(t *testing.T) {
	app, f := testkit.NewAppWithFixture(t)

	r := app.Client.Do(http.MethodGet, "/api/building/rent-control", f.TokenA, nil)
	r.RequireOK(t)

	type row struct {
		RoomNumber string  `json:"room_number"`
		Layout     string  `json:"layout"`
		RentPrice  float64 `json:"rent_price"`
		Remark     string  `json:"remark"`
	}
	var data struct {
		Building struct {
			ID      uint   `json:"id"`
			Name    string `json:"name"`
			Address string `json:"address"`
		} `json:"building"`
		Vacant   []row `json:"vacant"`
		Expiring []row `json:"expiring"`
		Total    int   `json:"total"`
	}
	r.Decode(t, &data)

	require.Equal(t, f.A.ID, data.Building.ID)
	require.Equal(t, "测试公寓A-全套餐", data.Building.Name)
	require.Equal(t, "南山区科技园测试花园A座", data.Building.Address)

	// 空房表：A101/A106/A107；A103 已预订，不导出。
	require.Len(t, data.Vacant, 3)
	require.Equal(t, []string{"A101", "A106", "A107"}, []string{
		data.Vacant[0].RoomNumber, data.Vacant[1].RoomNumber, data.Vacant[2].RoomNumber,
	})
	for i, item := range data.Vacant {
		require.Empty(t, item.Remark, "空房表第%d行不应有备注", i)
	}

	// 即将到期表：A104（5 天后到期），备注写到期时间；A102/A105 租期尚远，不导出。
	require.Len(t, data.Expiring, 1)
	require.Equal(t, "A104", data.Expiring[0].RoomNumber)
	require.Equal(t, "到期时间："+f.ContractA104.EndDate, data.Expiring[0].Remark)

	require.Equal(t, 4, data.Total)

	// 公寓隔离：B 栋只有 B201 空置可导出
	rb := app.Client.Do(http.MethodGet, "/api/building/rent-control", f.TokenB, nil)
	rb.RequireOK(t)
	var dataB struct {
		Vacant []row `json:"vacant"`
		Total  int   `json:"total"`
	}
	rb.Decode(t, &dataB)
	require.Equal(t, 1, dataB.Total)
	require.Len(t, dataB.Vacant, 1)
	require.Equal(t, "B201", dataB.Vacant[0].RoomNumber)

	// 未登录不可访问
	rn := app.Client.Do(http.MethodGet, "/api/building/rent-control", "", nil)
	rn.RequireError(t, http.StatusUnauthorized, 401)
}
