package entities

import (
	"database/sql/driver"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// RawJSON implements database/sql.Scanner and driver.Valuer to handle
// raw JSON / JSONB transparently without converting to escaped strings.
type RawJSON []byte

func (r RawJSON) MarshalJSON() ([]byte, error) {
	if len(r) == 0 {
		return []byte("null"), nil
	}
	return r, nil
}

func (r *RawJSON) UnmarshalJSON(data []byte) error {
	if r == nil {
		return errors.New("RawJSON: UnmarshalJSON on nil pointer")
	}
	*r = append((*r)[0:0], data...)
	return nil
}

func (r RawJSON) Value() (driver.Value, error) {
	if len(r) == 0 {
		return nil, nil
	}
	return string(r), nil
}

func (r *RawJSON) Scan(value any) error {
	if value == nil {
		*r = nil
		return nil
	}
	switch v := value.(type) {
	case []byte:
		*r = append((*r)[0:0], v...)
	case string:
		*r = []byte(v)
	default:
		return fmt.Errorf("cannot scan %T into RawJSON", value)
	}
	return nil
}

// AuditLog represents an immutable append-only record of an action performed in the system.
// NOTICE: It intentionally contains NO UpdatedAt and NO DeletedAt fields.
type AuditLog struct {
	ID         uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	ActorID    *uuid.UUID `gorm:"type:uuid;index" json:"actor_id,omitempty"`
	ActorPhone string     `gorm:"type:varchar(20)" json:"actor_phone"`
	ActorRole  string     `gorm:"type:varchar(50);not null;index" json:"actor_role"`
	ActorName  string     `gorm:"type:varchar(250)" json:"actor_name"`
	Action     string     `gorm:"type:varchar(100);not null;index" json:"action"`
	EntityType string     `gorm:"type:varchar(100);not null;index" json:"entity_type"`
	EntityID   string     `gorm:"type:varchar(100);index" json:"entity_id"`
	OldValues  RawJSON    `gorm:"type:jsonb" json:"old_values,omitempty"`
	NewValues  RawJSON    `gorm:"type:jsonb" json:"new_values,omitempty"`
	Reason     string     `gorm:"type:text" json:"reason,omitempty"`
	IPAddress  string     `gorm:"type:varchar(50)" json:"ip_address"`
	UserAgent  string     `gorm:"type:varchar(500)" json:"user_agent"`
	CreatedAt  time.Time  `gorm:"not null;default:CURRENT_TIMESTAMP;index" json:"created_at"`
}
