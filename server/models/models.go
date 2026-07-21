// models 包定义所有数据库模型（ORM 映射），以及自动迁移和清理函数。
package models

import (
	"time"

	"gorm.io/gorm"
)

// User 表示系统用户（管理员、超级管理员等）。
type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	DisplayName  string         `gorm:"size:50" json:"display_name"`
	Phone        string         `gorm:"size:20" json:"phone"`
	Email        string         `gorm:"size:100" json:"email"`
	Role         string         `gorm:"size:20;not null;default:'admin'" json:"role"`
	BuildingID   *uint          `gorm:"index" json:"building_id"`
	IsActive     bool           `gorm:"default:true" json:"is_active"`
	LastLoginAt  *time.Time     `json:"last_login_at"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

// Building 表示公寓楼栋，包含基本信息、套餐、地址等。
type Building struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"size:100;not null" json:"name"`
	Package      string         `gorm:"size:20;not null;default:'basic'" json:"package"`
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
	Status       string         `gorm:"size:20;not null;default:'active'" json:"status"`
	CreatedBy    uint           `json:"created_by"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
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
	ID                 uint           `gorm:"primaryKey" json:"id"`
	BuildingID         uint           `gorm:"index:idx_building_room;not null" json:"building_id"`
	RoomNumber         string         `gorm:"index:idx_building_room;size:20;not null" json:"room_number"`
	Floor              string         `gorm:"size:10;not null" json:"floor"`
	Layout             string         `gorm:"size:20;not null" json:"layout"`
	Area               float64        `gorm:"type:decimal(8,2)" json:"area"`
	Orientation        string         `gorm:"size:10" json:"orientation"`
	RentPrice            *float64 `gorm:"type:decimal(10,2)" json:"rent_price"`
	DepositMonths        *uint    `gorm:"default:0" json:"deposit_months"`
	ManagementFee        *float64 `gorm:"type:decimal(10,2)" json:"management_fee"`
	ElectricityUnitPrice *float64 `gorm:"type:decimal(10,4)" json:"electricity_unit_price"`
	WaterUnitPrice       *float64 `gorm:"type:decimal(10,4)" json:"water_unit_price"`
	Status             string         `gorm:"size:20;not null;default:'vacant'" json:"status"`
	Description        string         `gorm:"type:text" json:"description"`
	Media              []RoomMedia    `gorm:"foreignKey:RoomID" json:"media,omitempty"`
	CreatedAt          time.Time      `json:"created_at"`
	UpdatedAt          time.Time      `json:"updated_at"`
	DeletedAt          gorm.DeletedAt `gorm:"index" json:"-"`
}

// RoomMedia 表示房间的媒体资源（图片、视频等）。
type RoomMedia struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	RoomID        uint      `gorm:"index;not null;constraint:OnDelete:CASCADE" json:"room_id"`
	Type          string    `gorm:"size:10;not null" json:"type"`
	Category      string    `gorm:"size:20;not null;default:'gallery'" json:"category"`
	FilePath      string    `gorm:"size:500;not null" json:"file_path"`
	ThumbnailPath string    `gorm:"size:500" json:"thumbnail_path"`
	FileName      string    `gorm:"size:255" json:"file_name"`
	FileSize      int64     `json:"file_size"`
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
	ID           uint           `gorm:"primaryKey" json:"id"`
	RoomID       uint           `gorm:"index;not null;constraint:OnDelete:CASCADE" json:"room_id"`
	BuildingID   uint           `gorm:"index;not null" json:"building_id"`
	TenantID     uint           `gorm:"index;not null;constraint:OnDelete:CASCADE" json:"tenant_id"`
	RentPrice     float64       `gorm:"type:decimal(10,2);not null" json:"rent_price"`
	ManagementFee float64       `gorm:"type:decimal(10,2)" json:"management_fee"`
	Deposit       float64       `gorm:"type:decimal(10,2)" json:"deposit"`
	EarnestMoney  float64       `gorm:"type:decimal(10,2)" json:"earnest_money"`
	Prepaid       bool           `gorm:"default:false" json:"prepaid"`
	RentDay      uint           `gorm:"default:1" json:"rent_day"`
	PaymentCycle string         `gorm:"size:10;default:'monthly'" json:"payment_cycle"`
	ContractFile string         `gorm:"size:500" json:"contract_file"`
	StartDate    string         `gorm:"size:10;not null" json:"start_date"`
	EndDate      string         `gorm:"size:10" json:"end_date"`
	Status       string         `gorm:"size:10;not null;default:'active'" json:"status"`
	Room         Room           `gorm:"foreignKey:RoomID" json:"room,omitempty"`
	Tenant       Tenant         `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
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
	RoomID        *uint          `gorm:"index;constraint:OnDelete:SET NULL" json:"room_id"`
	Description   string         `gorm:"type:text" json:"description"`
	BillDate      string         `gorm:"size:10;not null" json:"bill_date"`
	CreatedBy     uint           `json:"created_by"`
	Room          Room           `gorm:"foreignKey:RoomID" json:"room,omitempty"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	DeletedAt     gorm.DeletedAt `gorm:"index" json:"-"`
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
	ShareholderID  uint           `gorm:"uniqueIndex:idx_dividend;not null;constraint:OnDelete:CASCADE" json:"shareholder_id"`
	DividendAmount float64        `gorm:"type:decimal(10,2)" json:"dividend_amount"`
	SettledBy      uint           `json:"settled_by"`
	PaidStatus     string         `gorm:"size:10;not null;default:'unpaid'" json:"paid_status"`
	PaidAt         *time.Time     `json:"paid_at"`
	Shareholder    Shareholder    `gorm:"foreignKey:ShareholderID" json:"shareholder,omitempty"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// Task 表示待办任务（维修、退租等）。
type Task struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	BuildingID  uint           `gorm:"index;not null" json:"building_id"`
	Title       string         `gorm:"size:200;not null" json:"title"`
	Type        string         `gorm:"size:50;not null" json:"type"`
	Priority    string         `gorm:"size:10;not null;default:'medium'" json:"priority"`
	Status      string         `gorm:"size:20;not null;default:'pending'" json:"status"`
	AssignedTo  string         `gorm:"size:50" json:"assigned_to"`
	DueDate     string         `gorm:"size:10" json:"due_date"`
	RoomID      *uint          `gorm:"index;constraint:OnDelete:SET NULL" json:"room_id"`
	Deposit     float64        `gorm:"type:decimal(10,2)" json:"deposit"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
	Room        Room           `gorm:"foreignKey:RoomID" json:"room,omitempty"`
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
	BuildingID uint      `gorm:"default:0;index:idx_pv_query" json:"building_id"`
	IP         string    `gorm:"size:45;not null" json:"ip"`
	CreatedAt  time.Time `gorm:"index:idx_pv_query;index:idx_pv_cleanup" json:"created_at"`
}

// AutoMigrate 自动创建或更新所有模型对应的数据库表。
func AutoMigrate(db *gorm.DB) error {
	// Room 的唯一索引改为普通索引（允许软删除后重建同号房间）
	if db.Migrator().HasIndex(&Room{}, "idx_building_room") {
		db.Migrator().DropIndex(&Room{}, "idx_building_room")
	}
	return db.AutoMigrate(
		&User{},
		&Building{},
		&BuildingLandlord{},
		&Room{},
		&RoomMedia{},
		&Tenant{},
		&RentalContract{},
		&Bill{},
		&Shareholder{},
		&Dividend{},
		&Task{},
		&Setting{},
		&RecruitSubmission{},
		&AuditLog{},
		&PageView{},
	)
}

// CleanupSoftDeleted 清理超过指定天数的软删除记录。
func CleanupSoftDeleted(db *gorm.DB, days int) error {
	cutoff := time.Now().AddDate(0, 0, -days)
	tables := []interface{}{
		&User{}, &Building{}, &Room{}, &RoomMedia{},
		&Tenant{}, &RentalContract{}, &Bill{},
		&Shareholder{}, &Dividend{}, &Task{},
	}
	for _, table := range tables {
		db.Unscoped().Where("deleted_at IS NOT NULL AND deleted_at < ?", cutoff).Delete(table)
	}
	return nil
}
