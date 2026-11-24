package entities

import (
	"github.com/google/uuid"
	"github.com/najm-x/nova/pkg/geo"
	"gorm.io/datatypes"
	"gorm.io/gorm"
)

type Stock struct {
	TimeStamp
	ID              uuid.UUID      `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Available       float64        `gorm:"type:numeric(10,2);not null;default:0"`
	PendingOrders   int            `gorm:"not null;default:0"`
	CompletedOrders int            `gorm:"not null;default:0"`
	Address         string         `gorm:"type:varchar(250);not null"`
	Location        geo.Point      `gorm:"type:geography(POINT,4326);not null"`
	StockTypeID     uint           `gorm:"not null;index"`
	StockType       StockType      `gorm:"foreignKey:StockTypeID"`
	Units           []Unit         `gorm:"foreignKey:StockID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	UserID          uuid.UUID      `gorm:"type:uuid;not null;index"`
	User            User           `gorm:"foreignKey:UserID"`
	StockDetails    datatypes.JSON `gorm:"type:jsonb"`
}

type StockType struct {
	gorm.Model
	Name           string       `gorm:"type:varchar(50);not null"`
	Slug           string       `gorm:"type:varchar(50);unique;not null"`
	ProviderTypeID uint         `gorm:"index"`
	ProviderType   ProviderType `gorm:"foreignKey:ProviderTypeID"`
}

type Unit struct {
	gorm.Model
	Name    string    `gorm:"type:varchar(50);not null"`
	Rate    int       `gorm:"not null;default:1"`
	Price   float64   `gorm:"type:numeric(10,2);not null;default:0"`
	IsMain  bool      `gorm:"not null;default:false"`
	StockID uuid.UUID `gorm:"type:uuid;not null;index"`
}

type Transaction struct {
	TimeStamp
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Amount      int       `gorm:"not null"`
	UnitID      uint      `gorm:"not null;index"`
	Unit        Unit      `gorm:"foreignKey:UnitID"`
	StockID     uuid.UUID `gorm:"type:uuid;not null;index"`
	Stock       Stock     `gorm:"foreignKey:StockID"`
	IsIn        bool      `gorm:"not null"`
	Description string    `gorm:"type:varchar(1000);default:null"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	User        User      `gorm:"foreignKey:UserID"`
}
