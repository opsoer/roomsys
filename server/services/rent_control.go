// rent_control.go 租控表数据聚合：只读查询，供前端生成图片。
//
// 数据来源全部是既有表，不新建任何表、不写入任何数据：
//   - buildings：公寓名称、地址、导航地址（directions）；
//   - rooms：房间号、格局、租金、管理费、状态；
//   - rental_contracts：活跃合同的到期日（判断「即将到期」）。
package services

import (
	"sort"
	"strings"

	"rental-server/models"
	"rental-server/utils"
)

// RentControlItem 租控表中的一行房间数据。
// 只保留图片上需要展示的列：房号、格局、租金、管理费；
// 备注列仅「即将到期」表格使用，空房表格不展示。
type RentControlItem struct {
	RoomNumber    string   `json:"room_number"`
	Layout        string   `json:"layout"`
	RentPrice     *float64 `json:"rent_price"`
	ManagementFee *float64 `json:"management_fee"`
	Remark        string   `json:"remark"`
	// 以下两项仅用于前端生成「收费标准」默认文案，不展示在表格列中
	ElectricityUnitPrice *float64 `json:"electricity_unit_price,omitempty"`
	WaterUnitPrice       *float64 `json:"water_unit_price,omitempty"`
}

// RentControlBuilding 租控表头部/底部需要展示的公寓信息。
type RentControlBuilding struct {
	ID         uint   `json:"id"`
	Name       string `json:"name"`
	Address    string `json:"address"`
	Directions string `json:"directions"`
}

// RentControlData 租控表聚合数据。
// Vacant / Expiring 分别对应图片上的两个独立表格：
//   - Vacant：空房表（房号、格局、租金、管理费，无备注列）；
//   - Expiring：即将到期表（多一列备注，写到期时间）。
type RentControlData struct {
	Building RentControlBuilding `json:"building"`
	Vacant   []RentControlItem   `json:"vacant"`
	Expiring []RentControlItem   `json:"expiring"`
	Total    int                 `json:"total"`
}

// RentControlData 聚合租控表数据：
//   - 未出租（vacant）房间归入 Vacant，图片上单独一张表；
//   - 已出租但合同即将到期（end_date 落在 [today, today+ExpiringSoonDays) 内）的
//     房间归入 Expiring，备注自动写「到期时间：YYYY-MM-DD」，便于提前招租；
//   - 两张表各自按房间号排序，与房间列表口径一致。
func (s *RoomService) RentControlData(buildingID uint) (*RentControlData, error) {
	var building models.Building
	if err := s.DB.First(&building, buildingID).Error; err != nil {
		return nil, err
	}

	// 未出租房间
	vacant := []models.Room{}
	if err := s.DB.Where("building_id = ? AND status = ?", buildingID, "vacant").
		Order("room_number ASC, id ASC").Find(&vacant).Error; err != nil {
		return nil, err
	}

	// 即将到期：活跃合同到期日在 [today, today+ExpiringSoonDays) 内。
	// 与 DynamicRoomStatus 的 expiring 判定保持同一阈值与区间口径。
	today := utils.Now().Format("2006-01-02")
	cutoff := utils.Now().AddDate(0, 0, utils.ExpiringSoonDays).Format("2006-01-02")
	contracts := []models.RentalContract{}
	if err := s.DB.Where("building_id = ? AND status = ? AND end_date != '' AND end_date >= ? AND end_date < ?",
		buildingID, "active", today, cutoff).Find(&contracts).Error; err != nil {
		return nil, err
	}
	endDateMap := make(map[uint]string, len(contracts))
	contractRoomIDs := make([]uint, 0, len(contracts))
	for _, ct := range contracts {
		contractRoomIDs = append(contractRoomIDs, ct.RoomID)
		endDateMap[ct.RoomID] = ct.EndDate
	}

	// 仅取状态仍为 rented 的房间，避免退租/其他异常状态下合同残留导致误导出
	expiring := []models.Room{}
	if len(contractRoomIDs) > 0 {
		if err := s.DB.Where("building_id = ? AND status = ? AND id IN ?",
			buildingID, "rented", contractRoomIDs).
			Order("room_number ASC, id ASC").Find(&expiring).Error; err != nil {
			return nil, err
		}
	}

	vacantItems := make([]RentControlItem, 0, len(vacant))
	for _, r := range vacant {
		vacantItems = append(vacantItems, RentControlItem{
			RoomNumber:           r.RoomNumber,
			Layout:               r.Layout,
			RentPrice:            r.RentPrice,
			ManagementFee:        r.ManagementFee,
			ElectricityUnitPrice: r.ElectricityUnitPrice,
			WaterUnitPrice:       r.WaterUnitPrice,
		})
	}
	sort.SliceStable(vacantItems, func(i, j int) bool { return vacantItems[i].RoomNumber < vacantItems[j].RoomNumber })

	expiringItems := make([]RentControlItem, 0, len(expiring))
	for _, r := range expiring {
		expiringItems = append(expiringItems, RentControlItem{
			RoomNumber:           r.RoomNumber,
			Layout:               r.Layout,
			RentPrice:            r.RentPrice,
			ManagementFee:        r.ManagementFee,
			Remark:               "到期时间：" + endDateMap[r.ID],
			ElectricityUnitPrice: r.ElectricityUnitPrice,
			WaterUnitPrice:       r.WaterUnitPrice,
		})
	}
	sort.SliceStable(expiringItems, func(i, j int) bool { return expiringItems[i].RoomNumber < expiringItems[j].RoomNumber })

	return &RentControlData{
		Building: RentControlBuilding{
			ID:         building.ID,
			Name:       building.Name,
			Address:    joinAddress(building.District, building.Street, building.Village, building.BuildingNo),
			Directions: building.Directions,
		},
		Vacant:   vacantItems,
		Expiring: expiringItems,
		Total:    len(vacantItems) + len(expiringItems),
	}, nil
}

// joinAddress 拼接公寓地址：过滤空白段后直接相连（中文地址无需分隔符）。
func joinAddress(parts ...string) string {
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if strings.TrimSpace(p) != "" {
			out = append(out, strings.TrimSpace(p))
		}
	}
	return strings.Join(out, "")
}
