package entities

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Stock struct {
	TimeStamp
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	Available   float64   `gorm:"type:numeric(10,2);not null;default:0"`
	StockTypeID uint      `gorm:"not null;index"`
	StockType   StockType `gorm:"foreignKey:StockTypeID"`
	Units       []Unit    `gorm:"foreignKey:StockID;constraint:OnUpdate:CASCADE,OnDelete:CASCADE;"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index"`
	User        User      `gorm:"foreignKey:UserID"`
}

type StockType struct {
	gorm.Model
	Name string `gorm:"type:varchar(50);not null"`
}

type Unit struct {
	gorm.Model
	Name    string    `gorm:"type:varchar(50);not null"`
	Rate    int       `gorm:"not null;default:1"`
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
