// Package models defines the GORM models of the Mawarid database.
//
// Source of truth: the approved ERD ("ERD 2", Diagrams/ERD 2.drawio) and the
// Sprint 1 scope in "Mawarid_Flows_Sprint1_Team_Handoff.docx".
//
// ERD tables are kept name/column compatible. Sprint 1 additions that the ERD
// does not model yet (sessions, OTP records, login attempts, audit log,
// identity documents, provider locations/working hours) are marked ADDITION
// and documented in docs/DB_SCHEMA.md. They do not alter any ERD column, so
// the future gas-ordering (Order) and inventory (stocks/units/…​) modules can
// be added on top without breaking changes.
package models

import (
	"time"

	"gorm.io/gorm"
)

// ---------------------------------------------------------------------------
// Account status & roles
// ---------------------------------------------------------------------------

// AccountStatus is the lifecycle state of an account (Sprint 1 requirement).
type AccountStatus string

const (
	// AccountStatusPendingApproval — registered, not yet allowed to transact.
	AccountStatusPendingApproval AccountStatus = "pending_approval"
	// AccountStatusActive — fully usable account.
	AccountStatusActive AccountStatus = "active"
	// AccountStatusRejected — administratively rejected.
	AccountStatusRejected AccountStatus = "rejected"
	// AccountStatusSuspended — temporarily blocked by an administrator.
	AccountStatusSuspended AccountStatus = "suspended"
)

// Valid reports whether s is a known account status.
func (s AccountStatus) Valid() bool {
	switch s {
	case AccountStatusPendingApproval, AccountStatusActive, AccountStatusRejected, AccountStatusSuspended:
		return true
	}
	return false
}

// Role name constants (RBAC).
const (
	RoleConsumer = "consumer"
	RoleProvider = "provider"
	RoleDriver   = "driver" // reserved — Sprint 2+
	RoleAdmin    = "admin"
)

// VerificationStatus is the state of the identity-document verification of a
// user (ADDITION — supports the SRS identity-verification workflow).
type VerificationStatus string

const (
	VerificationUnverified    VerificationStatus = "unverified"
	VerificationPendingReview VerificationStatus = "pending_review"
	VerificationVerified      VerificationStatus = "verified"
	VerificationRejected      VerificationStatus = "rejected"
)

// CustomerType mirrors the ERD Customer.type discriminator.
const (
	CustomerTypeHome     = "home"
	CustomerTypeBusiness = "business"
)

// User — ERD table `User`. One row per account (citizen, agent, driver, admin).
// The role determines which profile tables (customers/providers/drivers) apply.
type User struct {
	ID     uint `gorm:"primaryKey" json:"id"`
	RoleID uint `gorm:"not null;index" json:"role_id"`
	Role   Role `gorm:"foreignKey:RoleID" json:"-"`

	FullName    string `gorm:"type:varchar(120);not null" json:"full_name"`
	PhoneNumber string `gorm:"type:varchar(20);uniqueIndex:idx_users_phone;not null" json:"phone_number"`
	// PasswordHash — argon2id encoded string. NEVER serialized.
	PasswordHash string `gorm:"type:varchar(256);not null" json:"-"`
	// IsActive mirrors the ERD flag: soft account switch used together with
	// AccountStatus (false ⇒ blocked regardless of status).
	IsActive bool `gorm:"not null;default:true" json:"is_active"`

	// Identity information (ERD columns id_number / id_image_url).
	IDNumber   *string `gorm:"type:varchar(40)" json:"id_number,omitempty"`
	IDImageURL *string `gorm:"type:varchar(512)" json:"id_image_url,omitempty"`
	// UserType is a free label kept for ERD compatibility (gas/water later).
	UserType *string `gorm:"type:varchar(40)" json:"user_type,omitempty"`

	// Sprint 1 ADDITION: account lifecycle state.
	Status AccountStatus `gorm:"type:varchar(24);not null;default:'pending_approval';index" json:"status"`
	// Sprint 1 ADDITION: phone verification timestamp (set on successful OTP).
	PhoneVerifiedAt *time.Time `json:"phone_verified_at,omitempty"`
	// Sprint 1 ADDITION: identity-verification workflow state.
	VerificationStatus VerificationStatus `gorm:"type:varchar(24);not null;default:'unverified'" json:"verification_status"`
	// Sprint 1 ADDITION: administrative notes (shown to admins only).
	AdminNote *string `gorm:"type:text" json:"-"`

	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}

// Role — ERD table `Role`.
type Role struct {
	ID          uint         `gorm:"primaryKey" json:"id"`
	Name        string       `gorm:"type:varchar(40);uniqueIndex;not null" json:"name"`
	Permissions []Permission `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// Permission — ERD table `Permission`.
type Permission struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"type:varchar(80);uniqueIndex;not null" json:"name"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ---------------------------------------------------------------------------
// Profiles (ERD: Customer / Home / Businiss / Provider / Driver)
// ---------------------------------------------------------------------------

// Customer — ERD table `Customer` (1:1 with User, type home|business).
type Customer struct {
	UserID uint   `gorm:"primaryKey" json:"user_id"`
	User   *User  `gorm:"foreignKey:UserID" json:"-"`
	Type   string `gorm:"type:varchar(20);not null" json:"type"` // home | business
	// Subtype — SRS FR-CON-03 checkboxes.
	// home:     family | personal
	// business: restaurant | cafe | hotel | bakery | grocery
	Subtype   string    `gorm:"type:varchar(30);not null" json:"subtype"`
	Home      *Home     `gorm:"foreignKey:UserID" json:"home,omitempty"`
	Business  *Business `gorm:"foreignKey:UserID" json:"business,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Home — ERD table `Home` (customer subtype).
type Home struct {
	UserID      uint      `gorm:"primaryKey" json:"user_id"`
	FamilyCount *int      `gorm:"check:family_count >= 0" json:"family_count,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Business — ERD table `Businiss` (customer subtype). The ERD column is
// named `cmmercial_image_url`; the SQL migration keeps that exact spelling.
type Business struct {
	UserID uint `gorm:"primaryKey" json:"user_id"`
	// Commercial registry details — SRS FR-CON-03 ("معلومات السجل التجاري").
	CommercialRegistryInfo *string `gorm:"type:varchar(255)" json:"commercial_registry_info,omitempty"`
	// CommercialImageURL maps to the ERD/migration column "cmmercial_image_url"
	// (the ERD spelling is kept by 0001_init.up.sql); the explicit column tag
	// prevents GORM from generating the default "commercial_image_url", which
	// does not exist in the table (SQLSTATE 42703 on insert).
	CommercialImageURL *string   `gorm:"type:varchar(512);column:cmmercial_image_url" json:"commercial_image_url,omitempty"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

// Provider — ERD table `Provider` (1:1 with the agent's User row).
// Working hours start_time/end_time are ERD columns (TIME of day).
// Multiple distribution points / locations are Sprint 1 additions
// (provider_locations, provider_working_hours) kept in separate tables so the
// ERD row stays unchanged.
type Provider struct {
	UserID       uint   `gorm:"primaryKey" json:"user_id"`
	User         *User  `gorm:"foreignKey:UserID" json:"-"`
	ProviderName string `gorm:"type:varchar(120);not null" json:"provider_name"`
	// Description — SRS FR-PRV-01: free text, max 500 characters. Stored in the
	// migration column "provider_description" (the JSON field keeps the public
	// name provider_description); the explicit column tag prevents GORM from
	// generating the default "description", which does not exist in the table
	// (SQLSTATE 42703 on insert).
	Description *string   `gorm:"type:varchar(500);column:provider_description" json:"provider_description,omitempty"`
	StartTime   *string   `gorm:"type:varchar(8)" json:"start_time,omitempty"` // "HH:MM"
	EndTime     *string   `gorm:"type:varchar(8)" json:"end_time,omitempty"`   // "HH:MM"
	LogoURL     *string   `gorm:"type:varchar(512)" json:"logo_url,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Driver — ERD table `Driver` (reserved for Sprint 2; created for schema
// compatibility only, no driver API in Sprint 1).
type Driver struct {
	UserID    uint    `gorm:"primaryKey" json:"user_id"`
	CarNumber *string `gorm:"type:varchar(40)" json:"car_number,omitempty"`
	// Vehicle type — SRS FR-CON-03 ("نوع المركبة").
	VehicleType *string   `gorm:"type:varchar(60)" json:"vehicle_type,omitempty"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Address — ERD table `Address`. The ERD sketches a 1:1 link with User, but a
// citizen needs one delivery address per order later, so this is 1:N via
// user_id (superset of the ERD; no ERD column changed).
type Address struct {
	ID              uint           `gorm:"primaryKey" json:"id"`
	UserID          uint           `gorm:"not null;index" json:"user_id"`
	User            *User          `gorm:"foreignKey:UserID" json:"-"`
	City            string         `gorm:"type:varchar(80);not null" json:"city"`
	District        *string        `gorm:"type:varchar(80)" json:"district,omitempty"`
	Street          *string        `gorm:"type:varchar(255)" json:"street,omitempty"`
	AdditionalInfo  *string        `gorm:"type:varchar(255)" json:"additional_info,omitempty"`
	AddressImageURL *string        `gorm:"type:varchar(512)" json:"address_image_url,omitempty"`
	Latitude        *float64       `json:"latitude,omitempty"`
	Longitude       *float64       `json:"longitude,omitempty"`
	IsDefault       bool           `gorm:"not null;default:false" json:"is_default"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

// Notification — ERD table `Notification`.
type Notification struct {
	ID        uint       `gorm:"primaryKey" json:"id"`
	UserID    uint       `gorm:"not null;index" json:"user_id"`
	Title     string     `gorm:"type:varchar(200);not null" json:"title"`
	Content   string     `gorm:"type:text;not null" json:"content"`
	IsRead    bool       `gorm:"not null;default:false" json:"is_read"`
	ReadAt    *time.Time `json:"read_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
}

// ---------------------------------------------------------------------------
// Sprint 1 ADDITIONS — auth & security infrastructure
// ---------------------------------------------------------------------------

// OTPCode stores hashed one-time codes for phone verification and password
// recovery. The plaintext code is never persisted.
type OTPCode struct {
	ID          uint       `gorm:"primaryKey" json:"id"`
	PhoneNumber string     `gorm:"type:varchar(20);uniqueIndex:idx_otp_scope,priority:1;not null" json:"phone_number"`
	Purpose     string     `gorm:"type:varchar(24);uniqueIndex:idx_otp_scope,priority:2;not null" json:"purpose"` // register | login | password_reset
	CodeHash    string     `gorm:"type:varchar(128);not null" json:"-"`
	Attempts    int        `gorm:"not null;default:0" json:"attempts"`
	ConsumedAt  *time.Time `json:"consumed_at,omitempty"`
	ExpiresAt   time.Time  `gorm:"index;not null" json:"expires_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// OTP purposes.
const (
	OTPPurposeRegister      = "register"
	OTPPurposeLogin         = "login"
	OTPPurposePasswordReset = "password_reset"
)

// AuthSession stores refresh sessions (ADDITION). Access tokens are short-lived
// JWTs; refresh tokens are opaque random strings stored only as SHA-256 hashes
// so a database leak cannot be replayed.
type AuthSession struct {
	ID               string     `gorm:"type:varchar(64);primaryKey" json:"id"`
	UserID           uint       `gorm:"not null;index" json:"user_id"`
	User             *User      `gorm:"foreignKey:UserID" json:"-"`
	RefreshTokenHash string     `gorm:"type:varchar(64);uniqueIndex;not null" json:"-"`
	UserAgent        *string    `gorm:"type:varchar(255)" json:"user_agent,omitempty"`
	IP               *string    `gorm:"type:varchar(45)" json:"ip,omitempty"`
	ExpiresAt        time.Time  `gorm:"index;not null" json:"expires_at"`
	RevokedAt        *time.Time `json:"revoked_at,omitempty"`
	LastUsedAt       *time.Time `json:"last_used_at,omitempty"`
	CreatedAt        time.Time  `json:"created_at"`
}

// LoginAttempt is the persistent audit trail of login attempts (ADDITION).
// Combined with the Redis rate limiter it implements login attempt limiting.
type LoginAttempt struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	Phone         string    `gorm:"type:varchar(20);index:idx_login_attempts_scope,priority:1;not null" json:"phone"`
	IP            *string   `gorm:"type:varchar(45)" json:"ip,omitempty"`
	Success       bool      `gorm:"not null;default:false" json:"success"`
	FailureReason *string   `gorm:"type:varchar(60)" json:"failure_reason,omitempty"`
	CreatedAt     time.Time `gorm:"index:idx_login_attempts_scope,priority:2" json:"created_at"`
}

// IdentityDocument is an uploaded identity/commercial document (ADDITION).
// Files are stored on private disk; only metadata is exposed over the API.
type IdentityDocument struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	UserID       uint       `gorm:"not null;index" json:"user_id"`
	User         *User      `gorm:"foreignKey:UserID" json:"-"`
	DocumentType string     `gorm:"type:varchar(40);not null" json:"document_type"` // national_id | commercial_license | other
	FilePath     string     `gorm:"type:varchar(512);not null" json:"-"`
	OriginalName string     `gorm:"type:varchar(255);not null" json:"original_name"`
	ContentType  string     `gorm:"type:varchar(100);not null" json:"content_type"`
	SizeBytes    int64      `gorm:"not null" json:"size_bytes"`
	Sha256       string     `gorm:"type:varchar(64);index" json:"sha256"`
	Status       string     `gorm:"type:varchar(24);not null;default:'pending'" json:"status"` // pending | approved | rejected
	ReviewerID   *uint      `json:"reviewer_id,omitempty"`
	ReviewNote   *string    `gorm:"type:text" json:"review_note,omitempty"`
	ReviewedAt   *time.Time `json:"reviewed_at,omitempty"`
	CreatedAt    time.Time  `json:"created_at"`
	UpdatedAt    time.Time  `json:"updated_at"`
}

// AuditLog records privileged account actions (ADDITION). Log entries must not
// contain sensitive identity data (no id numbers, no document contents).
type AuditLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ActorUserID  *uint     `gorm:"index" json:"actor_user_id,omitempty"`
	TargetUserID *uint     `gorm:"index" json:"target_user_id,omitempty"`
	Action       string    `gorm:"type:varchar(80);not null;index" json:"action"`
	Details      *string   `gorm:"type:text" json:"details,omitempty"` // JSON, never PII of identity documents
	IP           *string   `gorm:"type:varchar(45)" json:"ip,omitempty"`
	CreatedAt    time.Time `gorm:"index" json:"created_at"`
}

// ---------------------------------------------------------------------------
// Sprint 1 ADDITIONS — provider locations & working hours
// ---------------------------------------------------------------------------

// ProviderLocation is a distribution point / service location of an agent
// (ADDITION — required by Sprint 1 "provider locations"; also the future
// anchor for distribution points in the gas-ordering module).
type ProviderLocation struct {
	ID             uint           `gorm:"primaryKey" json:"id"`
	ProviderID     uint           `gorm:"not null;index" json:"provider_id"` // Provider.UserID
	Provider       *Provider      `gorm:"foreignKey:ProviderID" json:"-"`
	Name           string         `gorm:"type:varchar(120);not null" json:"name"`
	City           string         `gorm:"type:varchar(80);not null" json:"city"`
	District       *string        `gorm:"type:varchar(80)" json:"district,omitempty"`
	Street         *string        `gorm:"type:varchar(255)" json:"street,omitempty"`
	AdditionalInfo *string        `gorm:"type:varchar(255)" json:"additional_info,omitempty"`
	Latitude       *float64       `json:"latitude,omitempty"`
	Longitude      *float64       `json:"longitude,omitempty"`
	IsActive       bool           `gorm:"not null;default:true" json:"is_active"`
	CreatedAt      time.Time      `json:"created_at"`
	UpdatedAt      time.Time      `json:"updated_at"`
	DeletedAt      gorm.DeletedAt `gorm:"index" json:"-"`
}

// ProviderWorkingHour is a weekly working-hour slot of a provider location
// (ADDITION — Sprint 1 "provider working hours").
type ProviderWorkingHour struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	ProviderID uint      `gorm:"not null;index" json:"provider_id"`
	LocationID *uint     `json:"location_id,omitempty"`
	DayOfWeek  int       `gorm:"not null;check:day_of_week between 0 and 6" json:"day_of_week"` // 0=Sunday … 6=Saturday
	StartTime  string    `gorm:"type:varchar(8);not null" json:"start_time"`                    // "HH:MM" 24h
	EndTime    string    `gorm:"type:varchar(8);not null" json:"end_time"`
	IsActive   bool      `gorm:"not null;default:true" json:"is_active"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
}

// AllModels returns every model for the migration runner / AutoMigrate.
func AllModels() []any {
	return []any{
		&Role{}, &Permission{}, &User{},
		&Customer{}, &Home{}, &Business{}, &Provider{}, &Driver{}, &Address{},
		&Notification{},
		&OTPCode{}, &AuthSession{}, &LoginAttempt{}, &IdentityDocument{}, &AuditLog{},
		&ProviderLocation{}, &ProviderWorkingHour{},
	}
}
