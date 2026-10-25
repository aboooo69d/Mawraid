package entities

type Role struct {
	ID          uint         `gorm:"primaryKey;autoIncrement" json:"id"`
	Name        string       `gorm:"type:varchar(50);uniqueIndex;not null" json:"name"`
	DisplayName string       `gorm:"type:varchar(100)" json:"display_name"`
	Permissions []Permission `gorm:"many2many:role_permissions;constraint:OnDelete:CASCADE;" json:"permissions,omitempty"`
}

type Permission struct {
	ID    uint   `gorm:"primaryKey;autoIncrement" json:"id"`
	Slug  string `gorm:"type:varchar(100);uniqueIndex;not null" json:"slug"`
	Name  string `gorm:"type:varchar(100);not null" json:"name"`
	Group string `gorm:"type:varchar(50);index" json:"group"`
}
