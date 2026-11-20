package repo

import (
	"context"
	"math"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/audit/dto"
	"gorm.io/gorm"
)

type (
	// AuditRepo enforces strict append-only operations.
	// It provides NO Update or Delete methods whatsoever.
	AuditRepo interface {
		Create(ctx context.Context, log *entities.AuditLog) error
		Find(ctx context.Context, filter dto.AuditLogFilter) ([]entities.AuditLog, int64, error)
		FindByID(ctx context.Context, id uuid.UUID) (*entities.AuditLog, error)
	}

	auditRepo struct {
		db *gorm.DB
	}
)

func NewAuditRepo(db *gorm.DB) AuditRepo {
	return &auditRepo{db: db}
}

// Create inserts a new immutable audit record into the database.
func (r *auditRepo) Create(ctx context.Context, log *entities.AuditLog) error {
	return r.db.WithContext(ctx).Create(log).Error
}

// Find retrieves filtered, paginated audit logs ordered strictly by newest first.
func (r *auditRepo) Find(ctx context.Context, filter dto.AuditLogFilter) ([]entities.AuditLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&entities.AuditLog{})

	if filter.ActorID != nil {
		query = query.Where("actor_id = ?", *filter.ActorID)
	}
	if filter.ActorRole != "" {
		query = query.Where("actor_role = ?", filter.ActorRole)
	}
	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}
	if filter.EntityType != "" {
		query = query.Where("entity_type = ?", filter.EntityType)
	}
	if filter.EntityID != "" {
		query = query.Where("entity_id = ?", filter.EntityID)
	}
	if filter.Search != "" {
		searchPattern := "%" + filter.Search + "%"
		query = query.Where(
			"reason ILIKE ? OR actor_name ILIKE ? OR actor_phone ILIKE ? OR entity_id ILIKE ?",
			searchPattern, searchPattern, searchPattern, searchPattern,
		)
	}
	if filter.FromDate != nil {
		query = query.Where("created_at >= ?", *filter.FromDate)
	}
	if filter.ToDate != nil {
		query = query.Where("created_at <= ?", *filter.ToDate)
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}

	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var logs []entities.AuditLog
	if err := query.Order("created_at DESC").Limit(pageSize).Offset(offset).Find(&logs).Error; err != nil {
		return nil, 0, err
	}

	return logs, total, nil
}

// FindByID retrieves a single audit record by its primary key UUID.
func (r *auditRepo) FindByID(ctx context.Context, id uuid.UUID) (*entities.AuditLog, error) {
	var log entities.AuditLog
	if err := r.db.WithContext(ctx).First(&log, "id = ?", id).Error; err != nil {
		return nil, err
	}
	return &log, nil
}

// CalculateTotalPages is a utility helper for pagination calculation.
func CalculateTotalPages(total int64, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	return int(math.Ceil(float64(total) / float64(pageSize)))
}
