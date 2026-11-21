package user_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	"github.com/najm-x/nova/middleware"
	"github.com/najm-x/nova/modules/user/dto"
	"github.com/najm-x/nova/modules/user/handler"
	userRepo "github.com/najm-x/nova/modules/user/repo"
	"github.com/najm-x/nova/modules/user/service"
	"github.com/najm-x/nova/pkg/respons"
	"gorm.io/gorm"
)

type mockUserRepo struct {
	usersByID          map[string]*entities.User
	usersByPhone       map[string]*entities.User
	providerTypes      map[uint]*entities.ProviderType
	nextProviderTypeID uint
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByID:          make(map[string]*entities.User),
		usersByPhone:       make(map[string]*entities.User),
		providerTypes:      make(map[uint]*entities.ProviderType),
		nextProviderTypeID: 1,
	}
}

func (m *mockUserRepo) CreateUser(c context.Context, user *entities.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	m.usersByID[user.ID.String()] = user
	m.usersByPhone[user.Phone] = user
	return nil
}

func (m *mockUserRepo) UpdateUser(c context.Context, id string, user *entities.User) error {
	existing, ok := m.usersByID[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	existing.Phone = user.Phone
	existing.Password = user.Password
	existing.IsVerified = user.IsVerified
	existing.Status = user.Status
	existing.RoleID = user.RoleID
	existing.Provider = user.Provider
	existing.Consumer = user.Consumer
	return nil
}

func (m *mockUserRepo) GetUserById(c context.Context, id string) (*entities.User, error) {
	u, ok := m.usersByID[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetUserByPhone(c context.Context, phone string) (*entities.User, error) {
	u, ok := m.usersByPhone[phone]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return u, nil
}

func (m *mockUserRepo) GetAllUsers(c context.Context, limit int, offset int) ([]entities.User, error) {
	list := make([]entities.User, 0, len(m.usersByID))
	for _, u := range m.usersByID {
		list = append(list, *u)
	}
	return list, nil
}

func (m *mockUserRepo) DeleteUser(c context.Context, id string) error {
	u, ok := m.usersByID[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	delete(m.usersByPhone, u.Phone)
	delete(m.usersByID, id)
	return nil
}

func (m *mockUserRepo) ListUsersByStatus(c context.Context, status entities.UserStatus, limit int, offset int) ([]entities.User, int64, error) {
	var list []entities.User
	for _, u := range m.usersByID {
		if u.Status == status {
			list = append(list, *u)
		}
	}
	return list, int64(len(list)), nil
}

func (m *mockUserRepo) UpdateUserStatus(c context.Context, id string, status entities.UserStatus) error {
	u, ok := m.usersByID[id]
	if !ok {
		return gorm.ErrRecordNotFound
	}
	u.Status = status
	return nil
}

func (m *mockUserRepo) CreateProviderType(c context.Context, pt *entities.ProviderType) error {
	for _, existing := range m.providerTypes {
		if strings.EqualFold(existing.Name, pt.Name) || strings.EqualFold(existing.Slug, pt.Slug) {
			return gorm.ErrDuplicatedKey
		}
	}
	if pt.ID == 0 {
		pt.ID = m.nextProviderTypeID
		m.nextProviderTypeID++
	}
	copyPt := *pt
	m.providerTypes[pt.ID] = &copyPt
	return nil
}

func (m *mockUserRepo) GetProviderTypes(c context.Context) ([]entities.ProviderType, error) {
	list := make([]entities.ProviderType, 0, len(m.providerTypes))
	for _, pt := range m.providerTypes {
		list = append(list, *pt)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].ID < list[j].ID
	})
	return list, nil
}

func (m *mockUserRepo) GetProviderTypeById(c context.Context, id uint) (*entities.ProviderType, error) {
	pt, ok := m.providerTypes[id]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	copyPt := *pt
	return &copyPt, nil
}

func (m *mockUserRepo) ProviderTypeExistsByNameOrSlug(c context.Context, name string, slug string) (bool, error) {
	for _, pt := range m.providerTypes {
		if strings.EqualFold(pt.Name, name) || strings.EqualFold(pt.Slug, slug) {
			return true, nil
		}
	}
	return false, nil
}

type mockStockRepo struct{}

func (m *mockStockRepo) GetStockByUserId(c context.Context, userId string) (*entities.Stock, error) {
	return nil, gorm.ErrRecordNotFound
}

var _ userRepo.UserRepo = (*mockUserRepo)(nil)

func TestProviderTypeService(t *testing.T) {
	ctx := context.Background()

	t.Run("CreateProviderType trims name and normalizes slug", func(t *testing.T) {
		repo := newMockUserRepo()
		svc := service.NewUserService(repo, &mockStockRepo{})

		res, err := svc.CreateProviderType(ctx, dto.CreateProviderTypeRequest{
			Name: "  Water Station  ",
			Slug: "  Water-Station ",
		})
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if res.ID != 1 {
			t.Fatalf("expected ID 1, got %d", res.ID)
		}
		if res.Name != "Water Station" {
			t.Fatalf("expected trimmed name 'Water Station', got %q", res.Name)
		}
		if res.Slug != "water-station" {
			t.Fatalf("expected normalized slug 'water-station', got %q", res.Slug)
		}
	})

	t.Run("CreateProviderType rejects duplicate name or slug case-insensitively", func(t *testing.T) {
		repo := newMockUserRepo()
		svc := service.NewUserService(repo, &mockStockRepo{})

		_, err := svc.CreateProviderType(ctx, dto.CreateProviderTypeRequest{
			Name: "Gas Station",
			Slug: "gas-station",
		})
		if err != nil {
			t.Fatalf("unexpected error creating initial type: %v", err)
		}

		_, err = svc.CreateProviderType(ctx, dto.CreateProviderTypeRequest{
			Name: "gas station",
			Slug: "other-slug",
		})
		if err != dto.ErrProviderTypeAlreadyExists {
			t.Fatalf("expected ErrProviderTypeAlreadyExists for duplicate name, got %v", err)
		}

		_, err = svc.CreateProviderType(ctx, dto.CreateProviderTypeRequest{
			Name: "Other Name",
			Slug: "GAS-STATION",
		})
		if err != dto.ErrProviderTypeAlreadyExists {
			t.Fatalf("expected ErrProviderTypeAlreadyExists for duplicate slug, got %v", err)
		}
	})

	t.Run("CreateProviderType rejects whitespace-only input", func(t *testing.T) {
		repo := newMockUserRepo()
		svc := service.NewUserService(repo, &mockStockRepo{})

		_, err := svc.CreateProviderType(ctx, dto.CreateProviderTypeRequest{
			Name: "   ",
			Slug: "valid-slug",
		})
		if err != respons.ErrBadRequest {
			t.Fatalf("expected ErrBadRequest, got %v", err)
		}
	})

	t.Run("GetProviderTypes returns non-nil empty slice and ordered items", func(t *testing.T) {
		repo := newMockUserRepo()
		svc := service.NewUserService(repo, &mockStockRepo{})

		emptyList, err := svc.GetProviderTypes(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if emptyList == nil || len(emptyList) != 0 {
			t.Fatalf("expected non-nil empty slice, got %#v", emptyList)
		}

		_, _ = svc.CreateProviderType(ctx, dto.CreateProviderTypeRequest{Name: "Gas", Slug: "gas"})
		_, _ = svc.CreateProviderType(ctx, dto.CreateProviderTypeRequest{Name: "Water", Slug: "water"})

		list, err := svc.GetProviderTypes(ctx)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if len(list) != 2 || list[0].ID != 1 || list[1].ID != 2 {
			t.Fatalf("unexpected list: %+v", list)
		}
	})

	t.Run("GetUserDetail includes ProviderTypeID and ProviderType", func(t *testing.T) {
		repo := newMockUserRepo()
		svc := service.NewUserService(repo, &mockStockRepo{})

		uid := uuid.New()
		pt := entities.ProviderType{
			Model: gorm.Model{ID: 7},
			Name:  "Fuel",
			Slug:  "fuel",
		}
		_ = repo.CreateUser(ctx, &entities.User{
			ID:     uid,
			Phone:  "771234567",
			RoleID: 2,
			Role:   entities.Role{ID: 2, Name: "provider"},
			Status: entities.StatusActive,
			Provider: &entities.Provider{
				UserID:         uid,
				Name:           "Main Fuel Provider",
				Description:    "Provides fuel",
				StartAt:        time.Now(),
				EndAt:          time.Now().Add(8 * time.Hour),
				ProviderTypeID: pt.ID,
				ProviderType:   pt,
			},
		})

		detail, err := svc.GetUserDetail(ctx, uid.String())
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if detail.Provider == nil {
			t.Fatal("expected provider info in detail response")
		}
		if detail.Provider.ProviderTypeID != 7 {
			t.Fatalf("expected provider_type_id 7, got %d", detail.Provider.ProviderTypeID)
		}
		if detail.Provider.ProviderType.ID != 7 || detail.Provider.ProviderType.Name != "Fuel" || detail.Provider.ProviderType.Slug != "fuel" {
			t.Fatalf("unexpected provider_type in detail response: %+v", detail.Provider.ProviderType)
		}
	})

	t.Run("UpdateUser updates provider_type_id when valid and returns 404 when missing", func(t *testing.T) {
		repo := newMockUserRepo()
		svc := service.NewUserService(repo, &mockStockRepo{})

		_ = repo.CreateProviderType(ctx, &entities.ProviderType{Name: "Gas", Slug: "gas"})
		_ = repo.CreateProviderType(ctx, &entities.ProviderType{Name: "Water", Slug: "water"})

		uid := uuid.New()
		_ = repo.CreateUser(ctx, &entities.User{
			ID:     uid,
			Phone:  "771112233",
			RoleID: 2,
			Role:   entities.Role{ID: 2, Name: "provider"},
			Provider: &entities.Provider{
				UserID:         uid,
				Name:           "My Provider Shop",
				ProviderTypeID: 1,
				ProviderType:   entities.ProviderType{Model: gorm.Model{ID: 1}, Name: "Gas", Slug: "gas"},
			},
		})

		missingID := uint(99)
		_, err := svc.UpdateUser(ctx, uid.String(), dto.UpdateUserRequest{
			Provider: &dto.UpdateProviderRequest{ProviderTypeID: &missingID},
		})
		if err != dto.ErrProviderTypeNotFound {
			t.Fatalf("expected ErrProviderTypeNotFound, got %v", err)
		}

		newID := uint(2)
		_, err = svc.UpdateUser(ctx, uid.String(), dto.UpdateUserRequest{
			Provider: &dto.UpdateProviderRequest{ProviderTypeID: &newID},
		})
		if err != nil {
			t.Fatalf("unexpected update error: %v", err)
		}

		updated, _ := repo.GetUserById(ctx, uid.String())
		if updated.Provider.ProviderTypeID != 2 || updated.Provider.ProviderType.Slug != "water" {
			t.Fatalf("expected provider type updated to 2/water, got %+v", updated.Provider)
		}
	})
}

func TestProviderTypeHandler(t *testing.T) {
	gin.SetMode(gin.TestMode)
	repo := newMockUserRepo()
	svc := service.NewUserService(repo, &mockStockRepo{})
	h := handler.NewUserHandler(svc)

	r := gin.New()
	r.Use(middleware.ErrorHandler())
	v1 := r.Group("/api/v1")
	v1.GET("/provider-types", h.ListProviderTypes)
	v1.POST("/admin/provider-types", h.CreateProviderType)

	t.Run("POST /admin/provider-types returns 400 on invalid payload", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/provider-types", bytes.NewBufferString(`{"name":"A"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d: %s", w.Code, w.Body.String())
		}
	})

	t.Run("POST /admin/provider-types returns 200 on valid payload and 409 on duplicate", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodPost, "/api/v1/admin/provider-types", bytes.NewBufferString(`{"name":"Water Delivery","slug":"water-delivery"}`))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var createResp struct {
			Success bool                     `json:"success"`
			Data    dto.ProviderTypeResponse `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &createResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !createResp.Success || createResp.Data.ID != 1 || createResp.Data.Slug != "water-delivery" {
			t.Fatalf("unexpected create response: %+v", createResp)
		}

		dupReq := httptest.NewRequest(http.MethodPost, "/api/v1/admin/provider-types", bytes.NewBufferString(`{"name":"Water Delivery","slug":"water-delivery"}`))
		dupReq.Header.Set("Content-Type", "application/json")
		dupW := httptest.NewRecorder()
		r.ServeHTTP(dupW, dupReq)

		if dupW.Code != http.StatusConflict {
			t.Fatalf("expected 409, got %d: %s", dupW.Code, dupW.Body.String())
		}
	})

	t.Run("GET /provider-types returns 200 with list", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/v1/provider-types", nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)

		if w.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
		}

		var listResp struct {
			Success bool                       `json:"success"`
			Data    []dto.ProviderTypeResponse `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &listResp); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}
		if !listResp.Success || len(listResp.Data) != 1 || listResp.Data[0].Name != "Water Delivery" {
			t.Fatalf("unexpected list response: %+v", listResp)
		}
	})
}
