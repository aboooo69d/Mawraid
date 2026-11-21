package auth_test

import (
	"context"
	"sort"
	"strings"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	userRepo "github.com/najm-x/nova/modules/user/repo"
	"gorm.io/gorm"
)

// mockUserRepo is an in-memory mock implementing userRepo.UserRepo.
type mockUserRepo struct {
	usersByPhone       map[string]*entities.User
	usersByID          map[string]*entities.User
	providerTypes      map[uint]*entities.ProviderType
	nextProviderTypeID uint
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByPhone:       make(map[string]*entities.User),
		usersByID:          make(map[string]*entities.User),
		providerTypes:      make(map[uint]*entities.ProviderType),
		nextProviderTypeID: 1,
	}
}

func (m *mockUserRepo) CreateUser(c context.Context, user *entities.User) error {
	if user.ID == uuid.Nil {
		user.ID = uuid.New()
	}
	m.usersByPhone[user.Phone] = user
	m.usersByID[user.ID.String()] = user
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
	var list []entities.User
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

var _ userRepo.UserRepo = (*mockUserRepo)(nil)

// mockRoleRepo is an in-memory mock for roleRepo.RoleRepo.
type mockRoleRepo struct {
	roles map[string]*entities.Role
}

func newMockRoleRepo() *mockRoleRepo {
	r := &mockRoleRepo{roles: make(map[string]*entities.Role)}
	r.roles[roleRepo.RoleConsumer] = &entities.Role{ID: 1, Name: roleRepo.RoleConsumer, DisplayName: "Consumer"}
	r.roles[roleRepo.RoleProvider] = &entities.Role{ID: 2, Name: roleRepo.RoleProvider, DisplayName: "Provider"}
	return r
}

func (m *mockRoleRepo) GetRoleByName(c context.Context, name string) (*entities.Role, error) {
	role, ok := m.roles[name]
	if !ok {
		return nil, gorm.ErrRecordNotFound
	}
	return role, nil
}

// mockSmsProvider captures sent SMS for testing.
type mockSmsProvider struct {
	sentMessages map[string][]string
	shouldFail   bool
}

func newMockSmsProvider() *mockSmsProvider {
	return &mockSmsProvider{
		sentMessages: make(map[string][]string),
	}
}

func (m *mockSmsProvider) SendSMS(phone string, message string) error {
	if m.shouldFail {
		return gorm.ErrInvalidDB
	}
	m.sentMessages[phone] = append(m.sentMessages[phone], message)
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

func (m *mockUserRepo) CreateProviderType(c context.Context, providerType *entities.ProviderType) error {
	for _, existing := range m.providerTypes {
		if strings.EqualFold(existing.Name, providerType.Name) || strings.EqualFold(existing.Slug, providerType.Slug) {
			return gorm.ErrDuplicatedKey
		}
	}
	if providerType.ID == 0 {
		providerType.ID = m.nextProviderTypeID
		m.nextProviderTypeID++
	}
	copyPt := *providerType
	m.providerTypes[providerType.ID] = &copyPt
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
