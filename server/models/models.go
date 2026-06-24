package models

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Username     string         `gorm:"uniqueIndex;size:50;not null" json:"username"`
	PasswordHash string         `gorm:"size:255;not null" json:"-"`
	Role         string         `gorm:"size:20;not null;default:'admin'" json:"role"`
	BuildingID   *uint          `gorm:"index" json:"building_id"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Building struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	Name         string         `gorm:"uniqueIndex;size:100;not null" json:"name"`
	ContractDate string         `gorm:"size:10" json:"contract_date"`
	ExpiredAt    string         `gorm:"size:10" json:"expired_at"`
	District     string         `gorm:"size:50" json:"district"`
	Street      string         `gorm:"size:100" json:"street"`
	Village     string         `gorm:"size:100" json:"village"`
	BuildingNo  string         `gorm:"size:50" json:"building_no"`
	CoverImage  string         `gorm:"size:500" json:"cover_image"`
	Description string         `gorm:"type:text" json:"description"`
	Status      string         `gorm:"size:20;not null;default:'active'" json:"status"`
	CreatedBy   uint           `json:"created_by"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type BuildingLandlord struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	BuildingID uint      `gorm:"index;not null" json:"building_id"`
	Name       string    `gorm:"size:50;not null" json:"name"`
	Phone      string    `gorm:"size:20" json:"phone"`
	CreatedAt  time.Time `json:"created_at"`
}

type Room struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	BuildingID  uint           `gorm:"uniqueIndex:idx_building_room;not null" json:"building_id"`
	RoomNumber  string         `gorm:"uniqueIndex:idx_building_room;size:20;not null" json:"room_number"`
	Floor       string         `gorm:"size:10;not null" json:"floor"`
	Layout      string         `gorm:"size:20;not null" json:"layout"`
	Status      string         `gorm:"size:20;not null;default:'vacant'" json:"status"`
	Description string         `gorm:"type:text" json:"description"`
	Media       []RoomMedia    `gorm:"foreignKey:RoomID" json:"media,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type RoomMedia struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	RoomID    uint      `gorm:"index;not null" json:"room_id"`
	Type      string    `gorm:"size:10;not null" json:"type"`
	Category  string    `gorm:"size:20;not null;default:'gallery'" json:"category"`
	FilePath  string    `gorm:"size:500;not null" json:"file_path"`
	FileName  string    `gorm:"size:255" json:"file_name"`
	FileSize  int64     `json:"file_size"`
	SortOrder int       `gorm:"default:0" json:"sort_order"`
	CreatedAt time.Time `json:"created_at"`
}

type Tenant struct {
	ID        uint           `gorm:"primaryKey" json:"id"`
	Name      string         `gorm:"size:50;not null" json:"name"`
	Phone     string         `gorm:"size:20" json:"phone"`
	IDCard    string         `gorm:"size:20" json:"id_card"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

type RentalContract struct {
	ID           uint           `gorm:"primaryKey" json:"id"`
	RoomID       uint           `gorm:"index;not null" json:"room_id"`
	BuildingID   uint           `gorm:"index;not null" json:"building_id"`
	TenantID     uint           `gorm:"index;not null" json:"tenant_id"`
	RentPrice    float64        `gorm:"type:decimal(10,2);not null" json:"rent_price"`
	Deposit      float64        `gorm:"type:decimal(10,2)" json:"deposit"`
	EarnestMoney float64        `gorm:"type:decimal(10,2)" json:"earnest_money"`
	StartDate    string         `gorm:"size:10;not null" json:"start_date"`
	EndDate      string         `gorm:"size:10" json:"end_date"`
	Status       string         `gorm:"size:10;not null;default:'active'" json:"status"`
	Room         Room           `gorm:"foreignKey:RoomID" json:"room,omitempty"`
	Tenant       Tenant         `gorm:"foreignKey:TenantID" json:"tenant,omitempty"`
	CreatedAt    time.Time      `json:"created_at"`
	UpdatedAt    time.Time      `json:"updated_at"`
	DeletedAt    gorm.DeletedAt `gorm:"index" json:"-"`
}

type Bill struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	BillNo      string         `gorm:"uniqueIndex;size:50;not null" json:"bill_no"`
	Type        string         `gorm:"size:10;not null" json:"type"`
	Subtype     string         `gorm:"size:30" json:"subtype"`
	Amount      float64        `gorm:"type:decimal(10,2);not null" json:"amount"`
	BuildingID  uint           `gorm:"index;not null" json:"building_id"`
	RoomID      *uint          `gorm:"index" json:"room_id"`
	Description string         `gorm:"type:text" json:"description"`
	BillDate    string         `gorm:"size:10;not null" json:"bill_date"`
	CreatedBy   uint           `json:"created_by"`
	Room        Room           `gorm:"foreignKey:RoomID" json:"room,omitempty"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	DeletedAt   gorm.DeletedAt `gorm:"index" json:"-"`
}

type Shareholder struct {
	ID         uint           `gorm:"primaryKey" json:"id"`
	BuildingID uint           `gorm:"index;not null" json:"building_id"`
	Name       string         `gorm:"size:50;not null" json:"name"`
	ShareRatio float64        `gorm:"type:decimal(5,2);not null" json:"share_ratio"`
	CreatedAt  time.Time      `json:"created_at"`
	UpdatedAt  time.Time      `json:"updated_at"`
	DeletedAt  gorm.DeletedAt `gorm:"index" json:"-"`
}

type Dividend struct {
	ID             uint        `gorm:"primaryKey" json:"id"`
	BuildingID     uint        `gorm:"index;not null" json:"building_id"`
	SettleMonth    string      `gorm:"size:7;not null" json:"settle_month"`
	TotalIncome    float64     `gorm:"type:decimal(10,2)" json:"total_income"`
	TotalExpense   float64     `gorm:"type:decimal(10,2)" json:"total_expense"`
	NetProfit      float64     `gorm:"type:decimal(10,2)" json:"net_profit"`
	ShareholderID  uint        `gorm:"index;not null" json:"shareholder_id"`
	DividendAmount float64     `gorm:"type:decimal(10,2)" json:"dividend_amount"`
	Shareholder    Shareholder `gorm:"foreignKey:ShareholderID" json:"shareholder,omitempty"`
	CreatedAt      time.Time   `json:"created_at"`
}

type Task struct {
	ID          uint           `gorm:"primaryKey" json:"id"`
	BuildingID  uint           `gorm:"index;not null" json:"building_id"`
	Title       string         `gorm:"size:200;not null" json:"title"`
	Type        string         `gorm:"size:50;not null" json:"type"`
	Status      string         `gorm:"size:20;not null;default:'pending'" json:"status"`
	RoomID      *uint          `gorm:"index" json:"room_id"`
	Deposit     float64        `gorm:"type:decimal(10,2)" json:"deposit"`
	Description string         `gorm:"type:text" json:"description"`
	CreatedAt   time.Time      `json:"created_at"`
	UpdatedAt   time.Time      `json:"updated_at"`
	Room        Room           `gorm:"foreignKey:RoomID" json:"room,omitempty"`
}

func AutoMigrate(db *gorm.DB) error {
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
	)
}
