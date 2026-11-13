package utils

import (
	"sync"
	"time"

	"rental-server/models"

	"gorm.io/gorm"
)

// aliveBuildingCacheTTL 存活楼栋缓存有效期。删除公寓时会主动失效；TTL 只用于兜底
// （多实例部署时其他实例的删除、绕过 BuildingService 的硬删等）。
const aliveBuildingCacheTTL = 30 * time.Second

type aliveBuildingCacheEntry struct {
	expiresAt time.Time
}

// aliveBuildingCache 只缓存「存活」正结果：map[buildingID]expiresAt。
// 新楼栋/已删楼栋不缓存，避免把「刚创建」误判为已删而丢数据。
var aliveBuildingCache sync.Map

// InvalidateBuildingAliveCache 使某个楼栋的存活缓存失效。删除公寓前后各调一次，
// 避免异步批次在 TTL 窗口内仍把它当作存活。
func InvalidateBuildingAliveCache(buildingID uint) {
	if buildingID != 0 {
		aliveBuildingCache.Delete(buildingID)
	}
}

// ClearBuildingAliveCache 清空存活缓存（测试隔离/运维重置用）。
func ClearBuildingAliveCache() {
	aliveBuildingCache.Range(func(key, _ interface{}) bool {
		aliveBuildingCache.Delete(key)
		return true
	})
}

// aliveBuildingIDs 返回 batch 涉及、且当前仍存活（未软删/未硬删）的 building_id 集合。
// 已缓存的正结果直接命中；未命中的一次性回源查询 buildings。
func aliveBuildingIDs(db *gorm.DB, batch []models.PageView) (map[uint]struct{}, error) {
	ids := make([]uint, 0, len(batch))
	seen := make(map[uint]struct{}, len(batch))
	for i := range batch {
		id := batch[i].BuildingID
		if id == 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return map[uint]struct{}{}, nil
	}

	alive := make(map[uint]struct{}, len(ids))
	missing := make([]uint, 0, len(ids))
	now := time.Now()
	for _, id := range ids {
		if v, ok := aliveBuildingCache.Load(id); ok {
			if e := v.(aliveBuildingCacheEntry); now.Before(e.expiresAt) {
				alive[id] = struct{}{}
				continue
			}
			aliveBuildingCache.Delete(id)
		}
		missing = append(missing, id)
	}
	if len(missing) == 0 {
		return alive, nil
	}

	var found []uint
	if err := db.Model(&models.Building{}).Where("id IN ?", missing).Pluck("id", &found).Error; err != nil {
		return nil, err
	}
	expiresAt := now.Add(aliveBuildingCacheTTL)
	for _, id := range found {
		alive[id] = struct{}{}
		aliveBuildingCache.Store(id, aliveBuildingCacheEntry{expiresAt: expiresAt})
	}
	return alive, nil
}
