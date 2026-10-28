package entities

import (
	"time"

	"github.com/google/uuid"
)

type UserStatus string

const (
	StatusPending   UserStatus = "pending"
	StatusDocs      UserStatus = "docs"
	StatusActive    UserStatus = "active"
	StatusRejected  UserStatus = "rejected"
	StatusSuspended UserStatus = "suspended"
)

type User struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Phone      string     `gorm:"type:varchar(20);not null;uniqueIndex:idx_users_phone,where:deleted_at IS NULL" json:"phone"`
	Password   string     `gorm:"type:varchar(255);not null" json:"-"`
	IsVerified bool       `gorm:"default:false" json:"is_verified"`
	Status     UserStatus `gorm:"type:user_status;default:'pending'" json:"status"`
	Role       Role       `gorm:"foreignKey:RoleID" json:"role"`
	RoleID     uint       `gorm:"not null" json:"role_id"`
	Provider   *Provider  `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"provider,omitempty"`
	Consumer   *Consumer  `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;" json:"consumer,omitempty"`

	TimeStamp
}

type Provider struct {
	UserID      uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	Name        string    `gorm:"type:varchar(250);not null" json:"name"`
	Description string    `gorm:"type:varchar(1000)" json:"description"`
	StartAt     time.Time `json:"start_at"`
	EndAt       time.Time `json:"end_at"`
	TimeStamp
}

type Consumer struct {
	UserID   uuid.UUID `gorm:"type:uuid;primaryKey" json:"user_id"`
	Name     string    `gorm:"type:varchar(250);not null" json:"name"`
	IdNumber string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_consumers_id_number,where:deleted_at IS NULL" json:"id_number"`
	IdImage  string    `gorm:"type:varchar(500)" json:"id_image"`

	IsDriver      bool    `gorm:"default:false" json:"is_driver"`
	VehicleNumber *string `gorm:"type:varchar(50)" json:"vehicle_number,omitempty"`
	VehicleType   *string `gorm:"type:varchar(50)" json:"vehicle_type,omitempty"`

	IsHome      bool `gorm:"default:false" json:"is_home"`
	FamilyCount *int `json:"family_count,omitempty"`

	IsBusiness   bool    `gorm:"default:false" json:"is_business"`
	BusinessName *string `gorm:"type:varchar(200)" json:"business_name,omitempty"`
	BusinessFile *string `gorm:"type:varchar(500)" json:"business_file,omitempty"`

	TimeStamp
}
