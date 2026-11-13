// 工具包，提供页面浏览统计和批量写入功能
package utils

import (
	"time"

	"rental-server/logger"
	"rental-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// pvChan 页面访问记录缓冲通道，容量 10000，避免阻塞请求
var pvChan = make(chan models.PageView, 10000)

// InitStatsWriter 启动后台协程，批量写入页面访问记录
func InitStatsWriter(db *gorm.DB) {
	go pvBatchWriter(db)
}

// pvBatchWriter 后台批量写入协程，每 10 秒或积累 100 条时写入一次
func pvBatchWriter(db *gorm.DB) {
	batch := make([]models.PageView, 0, 100)
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case pv := <-pvChan:
			batch = append(batch, pv)
			if len(batch) >= 100 {
				flushPVBatch(db, batch)
				batch = batch[:0]
			}
		case <-ticker.C:
			if len(batch) > 0 {
				flushPVBatch(db, batch)
				batch = batch[:0]
			}
		}
	}
}

// flushPVBatch 将一批页面访问记录写入数据库并同步维护聚合计数
func flushPVBatch(db *gorm.DB, batch []models.PageView) {
	if err := ApplyPageViewBatch(db, batch); err != nil {
		logger.Log.Warn().Err(err).Int("count", len(batch)).Msg("批量写入page_views与统计聚合失败")
	}
}

// ipDay 每日去重 IP 的 map 键；dailyKey 楼栋日计数的 map 键。
type ipDay struct{ day, ip string }

type dailyKey struct {
	day        string
	buildingID uint
}

// dailyAgg 单个 (day, building_id) 键的日增量。
type dailyAgg struct{ pv, detail, lv int64 }

// ApplyPageViewBatch 将一批页面访问记录落库并同步维护平台级聚合计数（单事务）。
//
// 五步：① 批量插入 page_views 明细（90 天定时清理语义不变，仍是楼栋级精确去重 UV、
// 房间排行、月排行 UV 的数据源）；② stat_ips 登记全历史去重 IP（总 UV 数据源，
// 新增 IP 数同步累加 stat_totals.total_uv）；③ upsert stat_totals 平台累计计数；
// ④ stat_daily_ips 登记当日去重 IP；⑤ upsert stat_daily 日计数——building_id>0
// 楼栋行与 building_id=0 平台行各累计一次。
//
// 平台级计数与公寓生死解耦：删除公寓只清明细与楼栋日行，stat_totals/stat_ips/
// stat_daily_ips 与 stat_daily 的平台行永不受影响，平台总数只增不减。
// 批次落库时若公寓已被删除（异步竞争窗口），该公寓的明细与楼栋行不再写回，
// 但平台级计数照常累计——事件真实发生过。
// 事务失败时整批丢弃（与历史 best-effort 语义一致），由调用方记日志，不重试。
func ApplyPageViewBatch(db *gorm.DB, batch []models.PageView) error {
	if len(batch) == 0 {
		return nil
	}
	return db.Transaction(func(tx *gorm.DB) error {
		// ⓪ 异步批次落库时公寓可能刚被删除（删除事务先提交、本批次后落库）：
		// 平台级计数照常累计（事件真实发生过，平台总量只增不减），
		// 仅跳过该公寓的明细行与楼栋日行，避免把孤儿数据写回。
		// 存活楼栋的正结果缓存 30 秒（删除公寓时主动失效），避免每批都查一次 buildings。
		aliveSet, err := aliveBuildingIDs(tx, batch)
		if err != nil {
			return err
		}
		// deadBuilding 返回 true 表示该公寓已被删除（软删/硬删），其事件不落明细与楼栋行
		deadBuilding := func(bid uint) bool {
			if bid == 0 {
				return false
			}
			_, ok := aliveSet[bid]
			return !ok
		}

		// ① 明细行（已删公寓的事件跳过）
		rows := make([]models.PageView, 0, len(batch))
		for i := range batch {
			if !deadBuilding(batch[i].BuildingID) {
				rows = append(rows, batch[i])
			}
		}
		if len(rows) > 0 {
			if err := tx.Create(&rows).Error; err != nil {
				return err
			}
		}

		now := time.Now()
		lvDelta := int64(0)
		ips := make(map[string]struct{}, len(batch))
		dayIPs := make(map[ipDay]struct{}, len(batch))
		daily := make(map[dailyKey]dailyAgg, len(batch)*2)

		for _, pv := range batch {
			day := pv.CreatedAt.Format("2006-01-02")
			isLandlordView := pv.PageType == "landlord_view"
			isDetail := pv.PageType == "building_detail" || pv.PageType == "room_detail"
			if isLandlordView {
				lvDelta++
			}
			if pv.IP != "" {
				ips[pv.IP] = struct{}{}
				dayIPs[ipDay{day: day, ip: pv.IP}] = struct{}{}
			}

			// 平台行（building_id=0）恒累计；楼栋行仅在公寓存活时累计
			keys := []dailyKey{{day: day, buildingID: 0}}
			if pv.BuildingID != 0 && !deadBuilding(pv.BuildingID) {
				keys = append(keys, dailyKey{day: day, buildingID: pv.BuildingID})
			}
			for _, k := range keys {
				agg := daily[k]
				agg.pv++
				if isDetail {
					agg.detail++
				}
				if isLandlordView {
					agg.lv++
				}
				daily[k] = agg
			}
		}

		// ② 全历史去重 IP（总 UV）：RowsAffected 即本次新增的 IP 数，
		// 只有真正插入的新 IP 才累加 stat_totals.total_uv（避免大表 COUNT(*)）。
		newIPs := int64(0)
		if len(ips) > 0 {
			rows := make([]models.StatIP, 0, len(ips))
			for ip := range ips {
				rows = append(rows, models.StatIP{IP: ip, CreatedAt: now})
			}
			res := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "ip"}},
				DoNothing: true,
			}).Create(&rows)
			if res.Error != nil {
				return res.Error
			}
			newIPs = res.RowsAffected
		}

		// ③ 平台累计计数（行不存在则初始化，存在则累加）
		if err := tx.Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "id"}},
			DoUpdates: clause.Assignments(map[string]interface{}{
				"total_pv":            gorm.Expr("total_pv + ?", int64(len(batch))),
				"total_uv":            gorm.Expr("total_uv + ?", newIPs),
				"total_landlord_view": gorm.Expr("total_landlord_view + ?", lvDelta),
				"updated_at":          now,
			}),
		}).Create(&models.StatTotal{ID: 1, TotalPV: int64(len(batch)), TotalUV: newIPs, TotalLandlordView: lvDelta, UpdatedAt: now}).Error; err != nil {
			return err
		}

		// ④ 当日去重 IP（每日平台 UV）
		if len(dayIPs) > 0 {
			rows := make([]models.StatDailyIP, 0, len(dayIPs))
			for k := range dayIPs {
				rows = append(rows, models.StatDailyIP{Day: k.day, IP: k.ip, CreatedAt: now})
			}
			if err := tx.Clauses(clause.OnConflict{
				Columns:   []clause.Column{{Name: "day"}, {Name: "ip"}},
				DoNothing: true,
			}).Create(&rows).Error; err != nil {
				return err
			}
		}

		// ⑤ 日计数：楼栋行 + 平台行（building_id=0）
		for k, agg := range daily {
			row := models.StatDaily{Day: k.day, BuildingID: k.buildingID, PV: agg.pv, DetailPV: agg.detail, LandlordView: agg.lv, UpdatedAt: now}
			if err := tx.Clauses(clause.OnConflict{
				Columns: []clause.Column{{Name: "day"}, {Name: "building_id"}},
				DoUpdates: clause.Assignments(map[string]interface{}{
					"pv":            gorm.Expr("pv + ?", agg.pv),
					"detail_pv":     gorm.Expr("detail_pv + ?", agg.detail),
					"landlord_view": gorm.Expr("landlord_view + ?", agg.lv),
					"updated_at":    now,
				}),
			}).Create(&row).Error; err != nil {
				return err
			}
		}
		return nil
	})
}

// RecordPageView 非阻塞投递一条页面访问记录到批量写入通道。
// 通道容量 10000；【满时静默丢弃该条记录】——避免高并发下阻塞请求，
// 因此 PV 统计为尽力而为，不保证每条都落库，勿作为可靠事件源依赖。
func RecordPageView(db *gorm.DB, pageType string, resourceID uint, buildingID uint, ip string) {
	select {
	case pvChan <- models.PageView{
		PageType:   pageType,
		ResourceID: resourceID,
		IP:         ip,
		BuildingID: buildingID,
		CreatedAt:  time.Now(),
	}:
	default:
	}
}

// GetRealIP 获取客户端真实 IP。
// 直接返回 c.ClientIP()，由 Gin 的 TrustedProxies 配置决定是否采信 X-Forwarded-For。
// 部署时需在 main.go 中调用 r.SetTrustedProxies() 指定可信反代 IP，防止伪造 XFF。
func GetRealIP(c *gin.Context) string {
	return c.ClientIP()
}
