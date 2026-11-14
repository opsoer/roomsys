// models 包定义所有数据库模型（ORM 映射），以及自动迁移和清理函数。
//
// 【外键与级联的约定（2026-11 修正）】
//
// GORM v2 只在**关联字段**（如 `Media []RoomMedia`、`Room Room`）上读取
// `constraint:OnDelete:...` 标签；写在**标量外键字段**（如 `RoomID uint`）上的
// 同名标签在全库没有任何消费点，是死配置。历史版本正是把标签写在了标量字段上，
// 导致建出来的外键一律是 `ON DELETE NO ACTION`，级联删除从未生效，
// 而清理逻辑又按「数据库会级联」的假设编写，导致删除公寓后大量数据永久残留。
//
// 因此本包的约定是：
//   - 级联语义写在关联字段上（新建库才会带上 ON DELETE 子句）；
//   - **存量库的外键仍是 NO ACTION**（结构变更不靠 AutoMigrate 猜，见 migrations 包），
//     所以所有「父删子」必须由业务代码显式完成，不能依赖数据库；
//   - 硬删除的先后顺序由 CleanupSoftDeleted 统一按「先子后父」编排。
package models

import (
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// User 表示系统用户（管理员、超级管理员等）。
type User struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	Username       string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash   string         `gorm:"size:255;not null" json:"-"`
	DisplayName    string         `gorm:"size:50" json:"display_name"`
	Phone          string         `gorm:"size:20" json:"phone"`
	Email          string         `gorm:"size:100" json:"email"`
	Role           string         `gorm:"size:20;not null;default:'admin'" json:"role"`
	BuildingID     *uint          `gorm:"index" json:"building_id"`
	IsActive       bool           `gorm:"default:true" json:"is_active"`
	TokenNotBefore *time.Time     `json:"-"`
	LastLoginAt    *time.Time     `json:"last_login_at"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// Building 表示公寓楼栋，包含基本信息、开放功能、地址等。
type Building struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	Package      string         `gorm:"size:20;not null;default:'basic'" json:"package"`
	Deposit      float64        `gorm:"type:decimal(12,2);default:0" json:"deposit"`
	ContractDate string         `gorm:"size:10" json:"contract_date"`
	ExpiredAt    string         `gorm:"size:10" json:"expired_at"`
	District     string         `gorm:"size:50" json:"district"`
	Street       string         `gorm:"size:100" json:"street"`
	Village      string         `gorm:"size:100" json:"village"`
	BuildingNo   string         `gorm:"size:50" json:"building_no"`
	TotalFloors  uint           `gorm:"default:1" json:"total_floors"`
	BankAccount  string         `gorm:"size:50" json:"bank_account"`
	CoverImage   string         `gorm:"size:500" json:"cover_image"`
	Description  string         `gorm:"type:text" json:"description"`
	Directions   string         `gorm:"type:text" json:"directions"`
	Status       string         `gorm:"size:20;not null;default:'active'" json:"status"`
	CreatedBy    uint           `json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
	// Landlords 房东列表，仅显式 Preload 时加载（BuildingWithStats 另有自己的 landlords 字段，序列化时同名外层优先）
	// constraint 写在关联字段上才会生效（标量字段上的同名 tag 是死配置）。
	Landlords []BuildingLandlord `gorm:"foreignKey:BuildingID;constraint:OnDelete:CASCADE" json:"landlords,omitempty"`
}

// BuildingLandlord 表示楼栋的房东信息。
type BuildingLandlord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	BuildingID uint      `gorm:"index;not null" json:"building_id"`
	Name       string    `gorm:"size:50;not null" json:"name"`
	Phone      string    `gorm:"size:20" json:"phone"`
	CreatedAt  time.Time `json:"created_at"`
}

// Room 表示公寓房间，包含布局、面积、租金定价等。
type Room struct {
	ID                   uint           `gorm:"primaryKey" json:"id"`
	BuildingID           uint           `gorm:"index:idx_building_room;not null" json:"building_id"`
	RoomNumber           string         `gorm:"index:idx_building_room;size:20;not null" json:"room_number"`
	Floor                string         `gorm:"size:10;not null" json:"floor"`
	Layout               string         `gorm:"size:20;not null" json:"layout"`
	Area                 float64        `gorm:"type:decimal(8,2)" json:"area"`
	Orientation          string         `gorm:"size:10" json:"orientation"`
	RentPrice            *float64       `gorm:"type:decimal(10,2)" json:"rent_price"`
	DepositMonths        *uint          `gorm:"default:0" json:"deposit_months"`
	ManagementFee        *float64       `gorm:"type:decimal(10,2)" json:"management_fee"`
	ElectricityUnitPrice *float64       `gorm:"type:decimal(10,4)" json:"electricity_unit_price"`
	WaterUnitPrice       *float64       `gorm:"type:decimal(10,4)" json:"water_unit_price"`
	Status               string         `gorm:"size:20;not null;default:'vacant'" json:"status"`
	Description          string         `gorm:"type:text" json:"description"`
	Media                []RoomMedia    `gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE" json:"media,omitempty"`
	CreatedAt            time.Time      `json:"created_at"`
	UpdatedAt            time.Time      `json:"updated_at"`
	DeletedAt            gorm.DeletedAt `gorm:"index" json:"-"`
}

// RoomMedia 表示房间的媒体资源（图片、视频等）。
type RoomMedia struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	RoomID        uint      `gorm:"index;not null" json:"room_id"`
	Type          string    `gorm:"size:10;not null" json:"type"`
	Category      string    `gorm:"size:20;not null;default:'gallery'" json:"category"`
	FilePath      string    `gorm:"size:500;not null" json:"file_path"`
	ThumbnailPath string    `gorm:"size:500" json:"thumbnail_path"`
	FileName      string    `gorm:"size:255" json:"file_name"`
	FileSize      int64     `json:"file_size"`
	Status        string    `gorm:"size:20;default:'ready'" json:"status"`
	TranscodeID   string    `gorm:"size:100" json:"-"`
	SortOrder     int       `gorm:"default:0" json:"sort_order"`
	CreatedAt     time.Time `json:"created_at"`
}

// Tenant 表示租客信息。
type Tenant struct {
	ID               uint           `gorm:"primaryKey" json:"id"`
	Name             string         `gorm:"size:50;not null" json:"name"`
	Phone            string         `gorm:"size:20" json:"phone"`
	IDCard           string         `gorm:"size:20" json:"id_card"`
	Email            string         `gorm:"size:100" json:"email"`
	EmergencyContact string         `gorm:"size:200" json:"emergency_contact"`
	CreatedAt        time.Time      `json:"created_at"`
	UpdatedAt        time.Time      `json:"updated_at"`
	DeletedAt        gorm.DeletedAt `gorm:"index" json:"-"`
}

// RentalContract 表示租赁合同，关联房间和租客。
type RentalContract struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	RoomID        uint           `gorm:"index;not null" json:"room_id"`
	BuildingID    uint           `gorm:"index;not null" json:"building_id"`
	TenantID      uint           `gorm:"index;not null" json:"tenant_id"`
	RentPrice     float64        `gorm:"type:decimal(10,2);not null" json:"rent_price"`
	ManagementFee float64        `gorm:"type:decimal(10,2)" json:"management_fee"`
	Deposit       float64        `gorm:"type:decimal(10,2)" json:"deposit"`
	EarnestMoney  float64        `gorm:"type:decimal(10,2)" json:"earnest_money"`
	Prepaid       bool           `gorm:"default:false" json:"prepaid"`
	RentDay       uint           `gorm:"default:1" json:"rent_day"`
	PaymentCycle  string         `gorm:"size:10;default:'monthly'" json:"payment_cycle"`
	ContractFile  string         `gorm:"size:500" json:"contract_file"`
	StartDate     string         `gorm:"size:10;not null" json:"start_date"`
	EndDate       string         `gorm:"size:10" json:"end_date"`
	Status        string         `gorm:"size:10;not null;default:'active'" json:"status"`
	Room          Room           `gorm:"foreignKey:RoomID;constraint:OnDelete:CASCADE" json:"room,omitempty"`
	Tenant        Tenant         `gorm:"foreignKey:TenantID;constraint:OnDelete:CASCADE" json:"tenant,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// Bill 表示账单，支持租金、水电等多种类型的费用记录。
type Bill struct {
	ID            uint           `gorm:"primaryKey" json:"id"`
	BillNo        string         `gorm:"uniqueIndex;size:50;not null" json:"bill_no"`
	Type          string         `gorm:"size:10;not null" json:"type"`
	Subtype       string         `gorm:"size:30" json:"subtype"`
	Amount        float64        `gorm:"type:decimal(10,2);not null" json:"amount"`
	PaidStatus    string         `gorm:"size:10;not null;default:'unpaid'" json:"paid_status"`
	PaidAt        *time.Time     `json:"paid_at"`
	PaymentMethod string         `gorm:"size:20" json:"payment_method"`
	BuildingID    uint           `gorm:"index;not null" json:"building_id"`
	RoomID        *uint          `gorm:"index" json:"room_id"`
	Description   string         `gorm:"type:text" json:"description"`
	BillDate      string         `gorm:"size:10;not null" json:"bill_date"`
	CreatedBy     uint           `json:"created_by"`
	Room          Room           `gorm:"foreignKey:RoomID;constraint:OnDelete:SET NULL" json:"room,omitempty"`
	Evidences     []BillEvidence `gorm:"foreignKey:BillID;constraint:OnDelete:CASCADE" json:"evidences,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
}

// BillEvidence 账单凭证截图：file_key 为对象存储 key（七牛 bucket 或本地 UploadDir 相对路径）。
// 删除语义：凭证行只随「管理员移除 / 更新账单时整单替换 / 账单被彻底删除」而删；
// 存储里的物理文件不即时删除，由定时 GC 统一清理「未被任何凭证行引用且超过宽限期」的孤儿文件。
type BillEvidence struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	BillID     uint      `gorm:"index;not null" json:"bill_id"`
	BuildingID uint      `gorm:"index;not null" json:"building_id"`
	FileKey    string    `gorm:"size:500;not null" json:"file_key"`
	FileName   string    `gorm:"size:255" json:"file_name"`
	FileSize   int64     `json:"file_size"`
	CreatedBy  uint      `json:"created_by"`
	CreatedAt  time.Time `json:"created_at"`
}

// Shareholder 表示楼栋股东及其持股比例。
type Shareholder struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	BuildingID uint           `gorm:"index;not null" json:"building_id"`
	Name       string         `gorm:"size:50;not null" json:"name"`
	ShareRatio float64        `gorm:"type:decimal(5,2);not null" json:"share_ratio"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// Dividend 表示月度分红记录，按股东计算分红金额。
type Dividend struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	BuildingID     uint           `gorm:"uniqueIndex:idx_dividend;not null" json:"building_id"`
	SettleMonth    string         `gorm:"uniqueIndex:idx_dividend;size:7;not null" json:"settle_month"`
	TotalIncome    float64        `gorm:"type:decimal(10,2)" json:"total_income"`
	TotalExpense   float64        `gorm:"type:decimal(10,2)" json:"total_expense"`
	NetProfit      float64        `gorm:"type:decimal(10,2)" json:"net_profit"`
	ShareholderID  uint           `gorm:"uniqueIndex:idx_dividend;not null" json:"shareholder_id"`
	DividendAmount float64        `gorm:"type:decimal(10,2)" json:"dividend_amount"`
	SettledBy      uint           `json:"settled_by"`
	PaidStatus     string         `gorm:"size:10;not null;default:'unpaid'" json:"paid_status"`
	PaidAt         *time.Time     `json:"paid_at"`
	Shareholder    Shareholder    `gorm:"foreignKey:ShareholderID;constraint:OnDelete:CASCADE" json:"shareholder,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// Task 表示待办任务（维修、退租、公寓到期提醒、招商申请等）。
// Scope 区分公寓级（building，房东后台可见）与平台级（platform，超级管理员可见）。
type Task struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	BuildingID  *uint          `gorm:"index" json:"building_id"`
	Title       string         `gorm:"size:200;not null" json:"title"`
	Type        string         `gorm:"size:50;not null" json:"type"`
	Priority    string         `gorm:"size:10;not null;default:'medium'" json:"priority"`
	Status      string         `gorm:"size:20;not null;default:'pending'" json:"status"`
	AssignedTo  string         `gorm:"size:50" json:"assigned_to"`
	DueDate     string         `gorm:"size:10" json:"due_date"`
	RoomID      *uint          `gorm:"index" json:"room_id"`
	Scope       string         `gorm:"size:20;not null;default:'building'" json:"scope"`
	RefID       *uint          `gorm:"index" json:"ref_id"`
	Deposit     float64        `gorm:"type:decimal(10,2)" json:"deposit"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Room        Room           `gorm:"foreignKey:RoomID;constraint:OnDelete:SET NULL" json:"room,omitempty"`
	Building    Building       `gorm:"foreignKey:BuildingID;constraint:OnDelete:CASCADE" json:"building,omitempty"`
}

// Setting 表示系统配置项（键值对）。
type Setting struct {
	Key       string    `gorm:"primaryKey;size:100" json:"key"`
	Value     string    `gorm:"type:text" json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// RecruitSubmission 表示招商提交的申请信息。
type RecruitSubmission struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Phone     string    `gorm:"size:20;not null" json:"phone"`
	Address   string    `gorm:"type:text;not null" json:"address"`
	Status    string    `gorm:"size:20;not null;default:'pending'" json:"status"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// AuditLog 表示操作审计日志，记录用户的关键操作。
type AuditLog struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index;not null" json:"user_id"`
	Username   string    `gorm:"size:50" json:"username"`
	Role       string    `gorm:"size:20" json:"role"`
	BuildingID *uint     `gorm:"index" json:"building_id"`
	Action     string    `gorm:"size:50;not null" json:"action"`
	Resource   string    `gorm:"size:50" json:"resource"`
	ResourceID string    `gorm:"size:50" json:"resource_id"`
	Detail     string    `gorm:"type:text" json:"detail"`
	IP         string    `gorm:"size:50" json:"ip"`
	CreatedAt  time.Time `json:"created_at"`
}

// PageView 表示页面浏览量记录，用于统计。
type PageView struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	PageType   string    `gorm:"size:30;not null;index:idx_pv_query" json:"page_type"`
	ResourceID uint      `gorm:"not null;default:0;index:idx_pv_query" json:"resource_id"`
	BuildingID uint      `gorm:"default:0;index:idx_pv_query;index:idx_pv_building" json:"building_id"`
	IP         string    `gorm:"size:45;not null" json:"ip"`
	CreatedAt  time.Time `gorm:"index:idx_pv_query;index:idx_pv_cleanup" json:"created_at"`
}

// StatTotal 平台级累计计数器（单行，主键固定为 1）：与具体公寓的生死解耦，只增不减。
// 总 PV/UV/获电数读取走本表（写入路径 utils.ApplyPageViewBatch 同步累加），
// 不再对 page_views/stat_ips 全表 COUNT(*)；删除公寓不会回退这里的值。
type StatTotal struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	TotalPV           int64     `gorm:"not null;default:0" json:"total_pv"`
	TotalUV           int64     `gorm:"not null;default:0" json:"total_uv"`
	TotalLandlordView int64     `gorm:"not null;default:0" json:"total_landlord_view"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// TableName 固定表名（避免 gorm 复数化歧义）。
func (StatTotal) TableName() string { return "stat_totals" }

// StatIP 全历史来过的去重 IP（每个 IP 一行，只增不减）：总 UV 的明细来源，
// 写入时新 IP 会同步累加 stat_totals.total_uv（避免大表 COUNT(*)）。
// 由 page_views 写入路径同步登记（INSERT OR IGNORE），与公寓删除、90 天明细清理均无关。
type StatIP struct {
	IP        string    `gorm:"primaryKey;size:45" json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 固定表名。
func (StatIP) TableName() string { return "stat_ips" }

// StatDailyIP 每日去重 IP（day+ip 复合主键）：当日平台 UV = 当日行数。
// 历史行由 soft_delete_cleanup 定时修剪（见 scheduler.AutoCleanupData，保留 370 天，
// 覆盖趋势接口最大 365 天窗口）。
type StatDailyIP struct {
	Day       string    `gorm:"primaryKey;size:10" json:"day"` // "2006-01-02"
	IP        string    `gorm:"primaryKey;size:45" json:"ip"`
	CreatedAt time.Time `json:"created_at"`
}

// TableName 固定表名。
func (StatDailyIP) TableName() string { return "stat_daily_ips" }

// StatDaily 按天按公寓的累计计数器（day+building_id 复合主键）：
//   - building_id=0 为平台级日计数（趋势图、今日/昨日 PV 的数据源，永不删除）；
//   - building_id>0  为楼栋日计数（楼栋累计 PV/获电数、月热度排行），随公寓删除一并清理。
//
// 列口径：PV=该公寓全部事件；DetailPV=building_detail+room_detail 两类（楼栋排行/
// 楼栋详情/楼栋概览获电率的历史分母口径，见 services/stats.go 注释）；LandlordView=获电埋点。
// 删除公寓只清 building_id=该公寓的行，平台行与其余公寓不受影响。
type StatDaily struct {
	Day          string    `gorm:"primaryKey;size:10" json:"day"` // "2006-01-02"
	BuildingID   uint      `gorm:"primaryKey;index" json:"building_id"`
	PV           int64     `gorm:"not null;default:0" json:"pv"`
	DetailPV     int64     `gorm:"not null;default:0" json:"detail_pv"`
	LandlordView int64     `gorm:"not null;default:0" json:"landlord_view"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// TableName 固定表名（避免 gorm 把 daily 复数化为 dailies）。
func (StatDaily) TableName() string { return "stat_daily" }

// BuildingRenewal 公寓入驻/续约记录，类似租约历史：
// join=入驻（记录签约日期与首次到期日），renew=续约（记录原到期日与新到期日）。
type BuildingRenewal struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	BuildingID uint           `gorm:"index;not null" json:"building_id"`
	Action     string         `gorm:"size:20;not null" json:"action"`
	FromDate   string         `gorm:"size:10" json:"from_date"`
	ToDate     string         `gorm:"size:10" json:"to_date"`
	Operator   string         `gorm:"size:50" json:"operator"`
	Note       string         `gorm:"size:200" json:"note"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// BuildingDepositLog 公寓押金变动流水，每条记录自成链上一环：
// create=创建公寓时的初始押金，adjust=平台调整（amount 正为增加、负为减少/扣罚）。
// before/after 记录变动前后余额，配合 reason（调整时由 handler 校验必填，
// 创建流水时可空）与 operator 可完整溯源每一次变动。
type BuildingDepositLog struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	BuildingID uint           `gorm:"index;not null" json:"building_id"`
	Action     string         `gorm:"size:20;not null" json:"action"`
	Amount     float64        `gorm:"type:decimal(12,2);not null" json:"amount"`
	Before     float64        `gorm:"type:decimal(12,2)" json:"before"`
	After      float64        `gorm:"type:decimal(12,2)" json:"after"`
	Reason     string         `gorm:"size:200;not null" json:"reason"`
	Operator   string         `gorm:"size:50" json:"operator"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

// LocationCustom 超级管理员在静态官方区划之外自定义的位置条目。
// Level: 1=区域(District 生效) 2=街道(District+Street) 3=村/小区(District+Street+Name)。
// Street 仅在 Level=2/3 时有意义；Level=1 时 District==Name。
type LocationCustom struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Level     int       `gorm:"not null;index:idx_loc_custom" json:"level"`
	District  string    `gorm:"size:50;not null;index:idx_loc_custom" json:"district"`
	Street    string    `gorm:"size:100;not null;default:'';index:idx_loc_custom" json:"street"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

// LocationRename 对静态官方区划条目的改名映射（自定义条目改名直接改自身记录，无需映射）。
// 级联已同步更新公寓表，此映射仅用于把静态基座里的旧名渲染成新名、避免新旧并存。
type LocationRename struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Level     int       `gorm:"not null;index:idx_loc_rename" json:"level"`
	District  string    `gorm:"size:50;not null;index:idx_loc_rename" json:"district"`
	Street    string    `gorm:"size:100;not null;default:'';index:idx_loc_rename" json:"street"`
	OldName   string    `gorm:"size:100;not null" json:"old_name"`
	NewName   string    `gorm:"size:100;not null" json:"new_name"`
	CreatedAt time.Time `json:"created_at"`
}

// SchemaMigration 记录已执行的数据库迁移版本（结构变更台账）。
type SchemaMigration struct {
	Version   int       `gorm:"primaryKey" json:"version"`
	Name      string    `gorm:"size:100;not null" json:"name"`
	AppliedAt time.Time `gorm:"not null" json:"applied_at"`
}

// AllModels 返回全部业务模型，用于建表与结构漂移校验。
func AllModels() []interface{} {
	return []interface{}{
		&User{},
		&Building{},
		&BuildingLandlord{},
		&BuildingRenewal{},
		&BuildingDepositLog{},
		&Room{},
		&RoomMedia{},
		&Tenant{},
		&RentalContract{},
		&Bill{},
		&BillEvidence{},
		&Shareholder{},
		&Dividend{},
		&Task{},
		&Setting{},
		&RecruitSubmission{},
		&AuditLog{},
		&PageView{},
		&StatTotal{},
		&StatIP{},
		&StatDailyIP{},
		&StatDaily{},
		&LocationCustom{},
		&LocationRename{},
	}
}

// AutoMigrate 自动创建或更新所有模型对应的数据库表。
//
// 注意：这是「无版本号」的结构同步，只能安全地用于**新增**表/列/索引；
// 字段改名、类型变更、数据回填等必须走 migrations 包的编号步骤，
// 否则历史数据不会被搬迁（详见 migrations 包说明）。
func AutoMigrate(db *gorm.DB) error {
	// Room 的唯一索引改为普通索引（允许软删除后重建同号房间）
	if db.Migrator().HasIndex(&Room{}, "idx_building_room") {
		db.Migrator().DropIndex(&Room{}, "idx_building_room")
	}
	return db.AutoMigrate(AllModels()...)
}

// CleanupSoftDeleted 清理超过指定天数的软删除记录（Unscoped 硬删除）。
//
// 【为什么不能简单按表名顺序删】（2026-11 修复）
//
//  1. 数据库外键在存量库里一律是 `ON DELETE NO ACTION`：父行只要还有子行指向它，
//     硬删父行就会报 `FOREIGN KEY constraint failed`。历史实现把错误全部丢弃且恒返回 nil，
//     定时任务因此永远显示「软删除: 完成」，实际上一条都没删掉。
//  2. 因此删除必须**先子后父**：待办→分红→股东→账单→合同→租客→房间→公寓→用户，
//     否则父行被子行挡住，子行又排在父行之后，本轮必然空转（下一轮才可能补上）。
//  3. bills.room_id / tasks.room_id 是「房间删掉后保留账单/待办」的有意设计，
//     软删房间时不会置空，硬删房间前必须显式断开，否则房间永远删不掉。
//  4. 无软删字段的子表（room_media / bill_evidences / building_landlords）随父行硬删而删。
//
// 返回值：任一步骤失败都会汇总进 error（不再静默丢弃），由调用方记录日志。
// 数据库时间比较用 time.Now()（无偏移）。
func CleanupSoftDeleted(db *gorm.DB, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	var errs []error

	// ① 先删无软删字段的子表行：账单凭证（随账单彻底删除而删）、房间媒体（随房间彻底删除而删）、
	//    房东（随公寓彻底删除而删）。同时清掉父行已不存在的孤儿行
	//    （历史库没有外键约束时，账单被硬删后凭证行会残留，进而把凭证文件永久钉住）。
	orphanSteps := []struct {
		name string
		sql  string
		args []interface{}
	}{
		{"账单凭证", `DELETE FROM bill_evidences WHERE bill_id IN (SELECT id FROM bills WHERE deleted_at IS NOT NULL AND deleted_at < ?)`, []interface{}{cutoff}},
		{"账单凭证(孤儿)", `DELETE FROM bill_evidences WHERE bill_id NOT IN (SELECT id FROM bills)`, nil},
		{"房间媒体", `DELETE FROM room_media WHERE room_id IN (SELECT id FROM rooms WHERE deleted_at IS NOT NULL AND deleted_at < ?)`, []interface{}{cutoff}},
		{"房间媒体(孤儿)", `DELETE FROM room_media WHERE room_id NOT IN (SELECT id FROM rooms)`, nil},
		{"房东", `DELETE FROM building_landlords WHERE building_id IN (SELECT id FROM buildings WHERE deleted_at IS NOT NULL AND deleted_at < ?)`, []interface{}{cutoff}},
		{"房东(孤儿)", `DELETE FROM building_landlords WHERE building_id NOT IN (SELECT id FROM buildings)`, nil},
	}
	for _, s := range orphanSteps {
		if err := db.Exec(s.sql, s.args...).Error; err != nil {
			errs = append(errs, fmt.Errorf("清理%s失败: %w", s.name, err))
		}
	}

	// ② 硬删房间前显式断开引用（软删房间时账单/待办被有意保留，room_id 仍指向房间）。
	//    用原生 SQL 而非 GORM：GORM 默认作用域会漏掉软删的账单与待办。
	detachSteps := []struct {
		name string
		sql  string
	}{
		{"账单房间引用", `UPDATE bills SET room_id = NULL WHERE room_id IN (SELECT id FROM rooms WHERE deleted_at IS NOT NULL AND deleted_at < ?)`},
		{"待办房间引用", `UPDATE tasks SET room_id = NULL WHERE room_id IN (SELECT id FROM rooms WHERE deleted_at IS NOT NULL AND deleted_at < ?)`},
		// 待办的公寓引用：公寓即将被彻底删除，待办本身保留（转成平台级待办）
		{"待办公寓引用", `UPDATE tasks SET building_id = NULL WHERE building_id IN (SELECT id FROM buildings WHERE deleted_at IS NOT NULL AND deleted_at < ?)`},
	}
	for _, s := range detachSteps {
		if err := db.Exec(s.sql, cutoff).Error; err != nil {
			errs = append(errs, fmt.Errorf("解除%s失败: %w", s.name, err))
		}
	}

	// ③ 按「先子后父」顺序硬删各软删表。
	tables := []struct {
		name  string
		model interface{}
	}{
		{"待办", &Task{}},                 // 引用 buildings / rooms
		{"分红", &Dividend{}},             // 引用 shareholders
		{"股东", &Shareholder{}},          //
		{"账单", &Bill{}},                 // 引用 rooms
		{"租赁合同", &RentalContract{}},     // 引用 rooms / tenants
		{"租客", &Tenant{}},               //
		{"房间", &Room{}},                 // 被 bills / tasks / contracts 引用
		{"入驻记录", &BuildingRenewal{}},    //
		{"押金流水", &BuildingDepositLog{}}, //
		{"公寓", &Building{}},             // 被 tasks / building_landlords 引用
		{"用户", &User{}},                 //
	}
	for _, t := range tables {
		if err := db.Unscoped().
			Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).
			Delete(t.model).Error; err != nil {
			errs = append(errs, fmt.Errorf("清理%s失败: %w", t.name, err))
		}
	}

	return errors.Join(errs...)
}
