package audit_test

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/modules/audit/dto"
	"github.com/najm-x/nova/modules/audit/repo"
	"github.com/najm-x/nova/modules/audit/service"
)

// mockAuditRepo provides an in-memory implementation of repo.AuditRepo
type mockAuditRepo struct {
	records []entities.AuditLog
}

func (m *mockAuditRepo) Create(ctx context.Context, log *entities.AuditLog) error {
	if log.ID == uuid.Nil {
		log.ID = uuid.New()
	}
	m.records = append(m.records, *log)
	return nil
}

func (m *mockAuditRepo) Find(ctx context.Context, filter dto.AuditLogFilter) ([]entities.AuditLog, int64, error) {
	var filtered []entities.AuditLog
	for _, r := range m.records {
		if filter.Action != "" && r.Action != filter.Action {
			continue
		}
		if filter.EntityType != "" && r.EntityType != filter.EntityType {
			continue
		}
		if filter.ActorRole != "" && r.ActorRole != filter.ActorRole {
			continue
		}
		filtered = append(filtered, r)
	}
	return filtered, int64(len(filtered)), nil
}

func (m *mockAuditRepo) FindByID(ctx context.Context, id uuid.UUID) (*entities.AuditLog, error) {
	for _, r := range m.records {
		if r.ID == id {
			return &r, nil
		}
	}
	return nil, nil
}

// TestAuditRepoImmutability ensures that the AuditRepo interface contains NO Update or Delete methods.
func TestAuditRepoImmutability(t *testing.T) {
	repoType := reflect.TypeOf((*repo.AuditRepo)(nil)).Elem()

	for i := 0; i < repoType.NumMethod(); i++ {
		method := repoType.Method(i)
		if method.Name == "Update" || method.Name == "Delete" || method.Name == "DeleteByID" || method.Name == "Truncate" {
			t.Fatalf("VIOLATION: AuditRepo must NEVER contain mutation or deletion method '%s'", method.Name)
		}
	}
}

// TestAuditService_Log verifies that audit events are correctly formatted and serialized.
func TestAuditService_Log(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := service.NewAuditService(mockRepo)

	actorUUID := uuid.New()
	oldState := map[string]any{"status": "pending", "price": 100}
	newState := map[string]any{"status": "active", "price": 120}

	input := dto.CreateAuditLogInput{
		ActorID:    &actorUUID,
		ActorPhone: "777777777",
		ActorRole:  "admin",
		ActorName:  "Super Admin",
		Action:     dto.ActionVerifyProvider,
		EntityType: dto.EntityProvider,
		EntityID:   "provider-uuid-123",
		OldValues:  oldState,
		NewValues:  newState,
		Reason:     "الوثائق مستوفية للشروط والتراخيص سارية",
		IPAddress:  "192.168.1.100",
		UserAgent:  "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
	}

	record, err := svc.Log(context.Background(), input)
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}

	if record.Action != dto.ActionVerifyProvider {
		t.Errorf("expected action %s, got %s", dto.ActionVerifyProvider, record.Action)
	}

	if record.ActorRole != "admin" {
		t.Errorf("expected role admin, got %s", record.ActorRole)
	}

	if record.Reason != input.Reason {
		t.Errorf("expected reason '%s', got '%s'", input.Reason, record.Reason)
	}

	// Verify JSON serialization of OldValues
	var parsedOld map[string]any
	if err := json.Unmarshal(record.OldValues, &parsedOld); err != nil {
		t.Fatalf("failed to unmarshal OldValues: %v", err)
	}
	if parsedOld["status"] != "pending" {
		t.Errorf("expected old status 'pending', got %v", parsedOld["status"])
	}

	// Verify JSON serialization of NewValues
	var parsedNew map[string]any
	if err := json.Unmarshal(record.NewValues, &parsedNew); err != nil {
		t.Fatalf("failed to unmarshal NewValues: %v", err)
	}
	if parsedNew["status"] != "active" {
		t.Errorf("expected new status 'active', got %v", parsedNew["status"])
	}

	// Verify in mock repo
	if len(mockRepo.records) != 1 {
		t.Fatalf("expected 1 record in repository, got %d", len(mockRepo.records))
	}
}

// TestAuditService_GetLogs verifies filtering and pagination calculation.
func TestAuditService_GetLogs(t *testing.T) {
	mockRepo := &mockAuditRepo{}
	svc := service.NewAuditService(mockRepo)

	ctx := context.Background()

	// Seed 3 logs
	_, _ = svc.Log(ctx, dto.CreateAuditLogInput{
		ActorRole:  "admin",
		Action:     dto.ActionAdminLogin,
		EntityType: dto.EntityAuth,
		Reason:     "تسجيل دخول ناجح",
	})
	_, _ = svc.Log(ctx, dto.CreateAuditLogInput{
		ActorRole:  "admin",
		Action:     dto.ActionVerifyProvider,
		EntityType: dto.EntityProvider,
		Reason:     "اعتماد مورد",
	})
	_, _ = svc.Log(ctx, dto.CreateAuditLogInput{
		ActorRole:  "provider",
		Action:     dto.ActionUpdateProfile,
		EntityType: dto.EntityProvider,
		Reason:     "تحديث رقم التواصل",
	})

	// 1. Fetch all
	res, err := svc.GetLogs(ctx, dto.AuditLogFilter{Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if res.Total != 3 {
		t.Errorf("expected total 3, got %d", res.Total)
	}

	// 2. Filter by Action
	resAction, err := svc.GetLogs(ctx, dto.AuditLogFilter{Action: dto.ActionVerifyProvider})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resAction.Total != 1 {
		t.Errorf("expected filtered total 1, got %d", resAction.Total)
	}

	// 3. Filter by Role
	resRole, err := svc.GetLogs(ctx, dto.AuditLogFilter{ActorRole: "provider"})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if resRole.Total != 1 {
		t.Errorf("expected filtered total 1, got %d", resRole.Total)
	}
}

// TestRawJSON_MarshalScan verifies SQL scanner and valuer logic for raw JSON.
func TestRawJSON_MarshalScan(t *testing.T) {
	sample := []byte(`{"key":"value","count":42}`)
	raw := entities.RawJSON(sample)

	val, err := raw.Value()
	if err != nil {
		t.Fatalf("Value() error: %v", err)
	}
	if val.(string) != string(sample) {
		t.Errorf("expected %s, got %v", string(sample), val)
	}

	var scanned entities.RawJSON
	if err := scanned.Scan(sample); err != nil {
		t.Fatalf("Scan() error: %v", err)
	}
	if string(scanned) != string(sample) {
		t.Errorf("expected %s, got %s", string(sample), string(scanned))
	}
}
