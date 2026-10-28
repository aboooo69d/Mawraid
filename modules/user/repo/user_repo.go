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
	return u.db.WithContext(c).Transaction(func(tx *gorm.DB) error {
		// Select(...) is required so zero values (false, 0, "") are written too.
		res := tx.Model(&entities.User{}).
			Where("id = ?", id).
			Select("Phone", "Password", "IsVerified", "IsActive", "RoleID").
			Updates(&entities.User{
				Phone:      user.Phone,
				Password:   user.Password,
				IsVerified: user.IsVerified,
				Status:     user.Status,
				RoleID:     user.RoleID,
			})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return gorm.ErrRecordNotFound
		}

		if user.Provider != nil {
			if err := tx.Model(&entities.Provider{}).
				Where("user_id = ?", id).
				Select("Name", "Description", "StartAt", "EndAt").
				Updates(user.Provider).Error; err != nil {
				return err
			}
		}

		if user.Consumer != nil {
			if err := tx.Model(&entities.Consumer{}).
				Where("user_id = ?", id).
				Select("Name", "IdNumber", "IdImage", "IsDriver", "VehicleNumber", "VehicleType",
					"IsHome", "FamilyCount", "IsBusiness", "BusinessName", "BusinessFile").
				Updates(user.Consumer).Error; err != nil {
				return err
			}
		}

		return nil
	})
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
