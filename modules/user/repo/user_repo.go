package repo

import (
	"context"

	"github.com/najm-x/nova/database/entities"
	"gorm.io/gorm"
)

type (
	UserRepo interface {
		CreateUser(c context.Context, user *entities.User) error
		UpdateUser(c context.Context, id string, user *entities.User) error
		GetUserById(c context.Context, id string) (*entities.User, error)
		GetUserByPhone(c context.Context, phone string) (*entities.User, error)
		GetAllUsers(c context.Context, limit int, offset int) ([]entities.User, error)
		DeleteUser(c context.Context, id string) error
	}

	userRepo struct {
		db *gorm.DB
	}
)

func NewUserRepo(db *gorm.DB) UserRepo {
	return &userRepo{db: db}
}

func (u *userRepo) CreateUser(c context.Context, user *entities.User) error {
	if err := gorm.G[entities.User](u.db).Create(c, user); err != nil {
		return err
	}
	return nil
}

func (u *userRepo) UpdateUser(c context.Context, id string, user *entities.User) error {
	rowsAffected, err := gorm.G[entities.User](u.db).Where("id = ?", id).Updates(c, *user)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (u *userRepo) GetUserById(c context.Context, id string) (*entities.User, error) {
	user, err := gorm.G[entities.User](u.db).
		Preload("Role", nil).
		Preload("Provider", nil).
		Preload("Consumer", nil).
		Where("id = ?", id).
		First(c)
	return &user, err
}

func (u *userRepo) GetUserByPhone(c context.Context, phone string) (*entities.User, error) {
	user, err := gorm.G[entities.User](u.db).
		Preload("Role", nil).
		Preload("Provider", nil).
		Preload("Consumer", nil).
		Where("phone = ?", phone).
		First(c)
	return &user, err
}

func (u *userRepo) GetAllUsers(c context.Context, limit int, offset int) ([]entities.User, error) {
	return gorm.G[entities.User](u.db).
		Preload("Role", nil).
		Preload("Provider", nil).
		Preload("Consumer", nil).
		Limit(limit).
		Offset(offset).
		Find(c)
}

func (u *userRepo) DeleteUser(c context.Context, id string) error {
	rowsAffected, err := gorm.G[entities.User](u.db).Where("id = ?", id).Delete(c)
	if err != nil {
		return err
	}
	if rowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}
