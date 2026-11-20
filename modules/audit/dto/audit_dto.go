package dto

import (
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
)

// Standard action constants
const (
	ActionAdminLogin       = "ADMIN_LOGIN"
	ActionAdminLogout      = "ADMIN_LOGOUT"
	ActionUserStatusUpdate = "USER_STATUS_UPDATE"
	ActionVerifyProvider   = "VERIFY_PROVIDER"
	ActionRejectProvider   = "REJECT_PROVIDER"
	ActionUpdateSetting    = "UPDATE_SETTING"
	ActionUpdateProfile    = "UPDATE_PROFILE"
	ActionResetPassword    = "RESET_PASSWORD"
)

// Standard entity type constants
const (
	EntityUser         = "users"
	EntityProvider     = "providers"
	EntityConsumer     = "consumers"
	EntityVerification = "verifications"
	EntitySetting      = "settings"
	EntityAuth         = "auth"
)

// CreateAuditLogInput is used by services to record an immutable audit event.
type CreateAuditLogInput struct {
	ActorID    *uuid.UUID `json:"actor_id,omitempty"`
	ActorPhone string     `json:"actor_phone"`
	ActorRole  string     `json:"actor_role"`
	ActorName  string     `json:"actor_name"`
	Action     string     `json:"action"`
	EntityType string     `json:"entity_type"`
	EntityID   string     `json:"entity_id"`
	OldValues  any        `json:"old_values,omitempty"`
	NewValues  any        `json:"new_values,omitempty"`
	Reason     string     `json:"reason,omitempty"`
	IPAddress  string     `json:"ip_address"`
	UserAgent  string     `json:"user_agent"`
}

// AuditLogFilter is used by handlers to filter and paginate audit records.
type AuditLogFilter struct {
	ActorID    *uuid.UUID `form:"actor_id"`
	ActorRole  string     `form:"actor_role"`
	Action     string     `form:"action"`
	EntityType string     `form:"entity_type"`
	EntityID   string     `form:"entity_id"`
	Search     string     `form:"search"`
	FromDate   *time.Time `form:"from_date" time_format:"2006-01-02T15:04:05Z07:00"`
	ToDate     *time.Time `form:"to_date" time_format:"2006-01-02T15:04:05Z07:00"`
	Page       int        `form:"page,default=1"`
	PageSize   int        `form:"page_size,default=20"`
}

// AuditLogListResponse wraps paginated audit logs.
type AuditLogListResponse struct {
	Logs       []entities.AuditLog `json:"logs"`
	Total      int64               `json:"total"`
	Page       int                 `json:"page"`
	PageSize   int                 `json:"page_size"`
	TotalPages int                 `json:"total_pages"`
}
