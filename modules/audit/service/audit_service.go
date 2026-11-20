package service

import (
	"context"
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/audit/dto"
	"github.com/najm-x/nova/modules/audit/repo"
)

type (
	AuditService interface {
		Log(ctx context.Context, input dto.CreateAuditLogInput) (*entities.AuditLog, error)
		GetLogs(ctx context.Context, filter dto.AuditLogFilter) (*dto.AuditLogListResponse, error)
		GetLogByID(ctx context.Context, id uuid.UUID) (*entities.AuditLog, error)
	}

	auditService struct {
		repo repo.AuditRepo
	}
)

func NewAuditService(repo repo.AuditRepo) AuditService {
	return &auditService{repo: repo}
}

// Log records an append-only audit entry. If serialization of OldValues or NewValues fails,
// it falls back gracefully without losing the audit event.
func (s *auditService) Log(ctx context.Context, input dto.CreateAuditLogInput) (*entities.AuditLog, error) {
	var oldRaw entities.RawJSON
	if input.OldValues != nil {
		if b, err := json.Marshal(input.OldValues); err == nil {
			oldRaw = entities.RawJSON(b)
		}
	}

	var newRaw entities.RawJSON
	if input.NewValues != nil {
		if b, err := json.Marshal(input.NewValues); err == nil {
			newRaw = entities.RawJSON(b)
		}
	}

	record := &entities.AuditLog{
		ActorID:    input.ActorID,
		ActorPhone: input.ActorPhone,
		ActorRole:  input.ActorRole,
		ActorName:  input.ActorName,
		Action:     input.Action,
		EntityType: input.EntityType,
		EntityID:   input.EntityID,
		OldValues:  oldRaw,
		NewValues:  newRaw,
		Reason:     input.Reason,
		IPAddress:  input.IPAddress,
		UserAgent:  input.UserAgent,
		CreatedAt:  time.Now().UTC(),
	}

	if err := s.repo.Create(ctx, record); err != nil {
		return nil, err
	}

	return record, nil
}

// GetLogs returns paginated audit records with computed total pages.
func (s *auditService) GetLogs(ctx context.Context, filter dto.AuditLogFilter) (*dto.AuditLogListResponse, error) {
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}
	filter.Page = page
	filter.PageSize = pageSize

	logs, total, err := s.repo.Find(ctx, filter)
	if err != nil {
		return nil, err
	}

	totalPages := repo.CalculateTotalPages(total, pageSize)

	return &dto.AuditLogListResponse{
		Logs:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetLogByID returns a single audit log entry.
func (s *auditService) GetLogByID(ctx context.Context, id uuid.UUID) (*entities.AuditLog, error) {
	return s.repo.FindByID(ctx, id)
}
