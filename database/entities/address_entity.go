package entities

import (
	"github.com/google/uuid"
	"github.com/najm-x/nova/pkg/geo"
)

// Address is a saved user address with a human description and a map location.
type Address struct {
	TimeStamp
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	User        User      `gorm:"foreignKey:UserID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	Title       string    `gorm:"type:varchar(100)"`
	Description string    `gorm:"type:varchar(250);not null"`
	Location    geo.Point `gorm:"type:geography(POINT,4326);not null"`
	IsDefault   bool      `gorm:"not null;default:false"`
}
