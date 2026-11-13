package auth_test

import (
	"context"

	"github.com/google/uuid"
	"github.com/najm-x/nova/database/entities"
	roleRepo "github.com/najm-x/nova/modules/role/repo"
	userRepo "github.com/najm-x/nova/modules/user/repo"
	"gorm.io/gorm"
)

// mockUserRepo is an in-memory mock implementing userRepo.UserRepo.
type mockUserRepo struct {
	usersByPhone map[string]*entities.User
	usersByID    map[string]*entities.User
}

func newMockUserRepo() *mockUserRepo {
	return &mockUserRepo{
		usersByPhone: make(map[string]*entities.User),
		usersByID:    make(map[string]*entities.User),
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

