// Package audit 提供写操作审计：统一格式写入日志文件（Info 级），
// 并尽力持久化到 audit_logs 表，便于事后追溯「谁在什么时候改了什么」。
//
// 记录范围约定（与日志级别配合）：
//   - 只记录写操作与敏感操作（增删改、状态变更、批量操作、管理员操作、配置变更）；
//   - 查询类操作不记录（需要排查时把日志级别调成 debug）；
//   - 失败路径由各自的 Error/Warn 日志负责，审计只记录已生效的结果。
package audit

import (
	"sort"
	"strconv"
	"sync"

	"rental-server/logger"
	"rental-server/models"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// 审计动作常量（按「资源.动作」命名，便于按前缀检索）。
const (
	// ActionCreate 新增。
	ActionCreate = "create"
	// ActionUpdate 修改。
	ActionUpdate = "update"
	// ActionDelete 删除。
	ActionDelete = "delete"
	// ActionStatus 状态变更。
	ActionStatus = "status"
	// ActionAudit 审核/处理。
	ActionAudit = "audit"
	// ActionSettle 结算/发放。
	ActionSettle = "settle"
	// ActionImport 批量导入。
	ActionImport = "import"
	// ActionTrigger 手动触发系统任务。
	ActionTrigger = "trigger"
	// ActionConfig 配置变更。
	ActionConfig = "config"
	// ActionLogin 登录。
	ActionLogin = "login"
	// ActionLogout 登出。
	ActionLogout = "logout"
)

// 审计资源常量。
const (
	ResourceUser        = "user"
	ResourceBuilding    = "building"
	ResourceLandlord    = "landlord"
	ResourceRoom        = "room"
	ResourceMedia       = "media"
	ResourceContract    = "contract"
	ResourceBill        = "bill"
	ResourceShareholder = "shareholder"
	ResourceDividend    = "dividend"
	ResourceTask        = "task"
	ResourceRecruit     = "recruit"
	ResourceSettings    = "settings"
	ResourceLocation    = "location"
	ResourceTime        = "time"
	ResourceJob         = "job"
)

var (
	mu sync.RWMutex
	db *gorm.DB
)

// Init 注入数据库句柄（进程启动时调用一次），用于把审计记录持久化。
func Init(database *gorm.DB) {
	mu.Lock()
	db = database
	mu.Unlock()
}

func dbHandle() *gorm.DB {
	mu.RLock()
	defer mu.RUnlock()
	return db
}

// Log 记录一次写操作审计。c 为请求上下文（可为 nil，用于非 HTTP 场景），
// action/resource 形如 "room.update"，resourceID 为业务主键（字符串）。
func Log(c *gin.Context, action, resource, resourceID, detail string) {
	var (
		actorID    uint
		actorName  = "system"
		role       = "system"
		ip         string
		buildingID *uint
	)
	if c != nil {
		actorID = c.GetUint("user_id")
		if v := c.GetString("username"); v != "" {
			actorName = v
		}
		if v := c.GetString("role"); v != "" {
			role = v
		}
		ip = c.ClientIP()
		if bid := c.GetUint("building_id"); bid > 0 {
			b := bid
			buildingID = &b
		}
	}
	write(actorID, actorName, role, action, resource, resourceID, detail, ip, buildingID)
}

// LogSystem 记录系统级操作（定时任务执行、启动、配置变更等），无操作人。
func LogSystem(action, resource, resourceID, detail string) {
	write(0, "system", "system", action, resource, resourceID, detail, "", nil)
}

// LogAs 以指定身份记录审计（用于登录等尚未写入请求上下文的场景）。
func LogAs(actorID uint, actorName, role, action, resource, resourceID, detail string) {
	write(actorID, actorName, role, action, resource, resourceID, detail, "", nil)
}

// write 输出审计日志并尽力落库（落库失败只告警，不影响业务响应）。
func write(actorID uint, actorName, role, action, resource, resourceID, detail, ip string, buildingID *uint) {
	ev := logger.Log.Info().
		Str("action", action).
		Str("resource", resource)
	if resourceID != "" {
		ev = ev.Str("resource_id", resourceID)
	}
	if actorID > 0 {
		ev = ev.Uint("actor_id", actorID)
	}
	if actorName != "" {
		ev = ev.Str("actor", actorName)
	}
	if role != "" {
		ev = ev.Str("role", role)
	}
	if ip != "" {
		ev = ev.Str("ip", ip)
	}
	if detail != "" {
		ev = ev.Str("detail", detail)
	}
	ev.Msg("操作审计")

	persist(actorID, actorName, action, resource, resourceID, detail, ip, buildingID)
}

// persist 把审计记录写入 audit_logs 表（尽力而为）。
func persist(actorID uint, actorName, action, resource, resourceID, detail, ip string, buildingID *uint) {
	d := dbHandle()
	if d == nil {
		return
	}
	rec := &models.AuditLog{
		UserID:     actorID,
		Username:   actorName,
		Action:     action,
		Resource:   resource,
		ResourceID: resourceID,
		Detail:     detail,
		IP:         ip,
		BuildingID: buildingID,
	}
	if err := d.Create(rec).Error; err != nil {
		logger.Log.Warn().Err(err).Str("action", action).Msg("审计记录落库失败")
	}
}

// ID 辅助函数：把主键转成字符串，便于统一记录 resource_id。
func ID(id uint) string {
	return strconv.FormatUint(uint64(id), 10)
}

// Keys 返回 map 的键名（已排序），用于审计只记录「改了哪些字段」而不记录字段值。
func Keys(m map[string]interface{}) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
