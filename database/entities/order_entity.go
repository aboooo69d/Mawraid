package entities

import (
	"github.com/google/uuid"
	"gorm.io/datatypes"
)

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusAccepted  OrderStatus = "accepted"
	OrderStatusCompleted OrderStatus = "completed"
	OrderStatusRejected  OrderStatus = "rejected"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type OrderConsumerType string

const (
	OrderConsumerHome     OrderConsumerType = "home"
	OrderConsumerBusiness OrderConsumerType = "business"
	OrderConsumerDriver   OrderConsumerType = "driver"
)

type PaymentMethod string

const (
	PaymentMethodCash   PaymentMethod = "cash"
	PaymentMethodWallet PaymentMethod = "wallet"
)

type PaymentStatus string

const (
	PaymentStatusUnpaid   PaymentStatus = "unpaid"
	PaymentStatusPaid     PaymentStatus = "paid"
	PaymentStatusRefunded PaymentStatus = "refunded"
)

type Order struct {
	TimeStamp
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()"`
	OrderNumber string    `gorm:"type:varchar(32);uniqueIndex;not null"`

	ConsumerID   uuid.UUID         `gorm:"type:uuid;not null;index"`
	Consumer     User              `gorm:"foreignKey:ConsumerID"`
	ProviderID   uuid.UUID         `gorm:"type:uuid;not null;index"`
	Provider     User              `gorm:"foreignKey:ProviderID"`
	StockID      uuid.UUID         `gorm:"type:uuid;not null;index"`
	Stock        Stock             `gorm:"foreignKey:StockID"`
	AddressID    uuid.UUID         `gorm:"type:uuid;not null;index"`
	Address      Address           `gorm:"foreignKey:AddressID"`
	ConsumerType OrderConsumerType `gorm:"type:varchar(20);not null"`

	// Unit & price snapshot at booking time
	UnitID     uint    `gorm:"not null"`
	Unit       Unit    `gorm:"foreignKey:UnitID"`
	UnitName   string  `gorm:"type:varchar(50);not null"`
	UnitRate   int     `gorm:"not null"`
	UnitPrice  float64 `gorm:"type:numeric(10,2);not null"`
	Quantity   int     `gorm:"not null"`
	TotalPrice float64 `gorm:"type:numeric(10,2);not null"`

	// Status & payment
	Status        OrderStatus   `gorm:"type:varchar(20);not null;default:'pending';index"`
	PaymentMethod PaymentMethod `gorm:"type:varchar(20);not null;default:'cash'"`
	PaymentStatus PaymentStatus `gorm:"type:varchar(20);not null;default:'unpaid'"`

	// Notes, reasons & stock transaction link
	Notes         string         `gorm:"type:varchar(500)"`
	Reason        string         `gorm:"type:varchar(500)"`
	CancelledBy   *uuid.UUID     `gorm:"type:uuid"`
	TransactionID *uuid.UUID     `gorm:"type:uuid;index"`
	OrderDetails  datatypes.JSON `gorm:"type:jsonb"`
}
