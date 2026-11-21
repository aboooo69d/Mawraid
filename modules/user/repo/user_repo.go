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
		ListUsersByStatus(c context.Context, status entities.UserStatus, limit int, offset int) ([]entities.User, int64, error)
		UpdateUserStatus(c context.Context, id string, status entities.UserStatus) error
		CreateProviderType(c context.Context, providerType *entities.ProviderType) error
		GetProviderTypes(c context.Context) ([]entities.ProviderType, error)
		GetProviderTypeById(c context.Context, id uint) (*entities.ProviderType, error)
		ProviderTypeExistsByNameOrSlug(c context.Context, name string, slug string) (bool, error)
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
			Select("Phone", "Password", "IsVerified", "Status", "RoleID").
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
			fields := []string{"Name", "Description", "StartAt", "EndAt"}
			if user.Provider.ProviderTypeID != 0 {
				fields = append(fields, "ProviderTypeID")
			}
			if err := tx.Model(&entities.Provider{}).
				Where("user_id = ?", id).
				Select(fields).
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
		Preload("Provider.ProviderType", nil).
		Preload("Consumer", nil).
		Where("id = ?", id).
		First(c)
	return &user, err
}

func (u *userRepo) GetUserByPhone(c context.Context, phone string) (*entities.User, error) {
	user, err := gorm.G[entities.User](u.db).
		Preload("Role", nil).
		Preload("Provider", nil).
		Preload("Provider.ProviderType", nil).
		Preload("Consumer", nil).
		Where("phone = ?", phone).
		First(c)
	return &user, err
}

func (u *userRepo) GetAllUsers(c context.Context, limit int, offset int) ([]entities.User, error) {
	return gorm.G[entities.User](u.db).
		Preload("Role", nil).
		Preload("Provider", nil).
		Preload("Provider.ProviderType", nil).
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

// ListUsersByStatus returns one page of users with the given status (newest first) and the total count.
func (u *userRepo) ListUsersByStatus(c context.Context, status entities.UserStatus, limit int, offset int) ([]entities.User, int64, error) {
	var total int64
	if err := u.db.WithContext(c).Model(&entities.User{}).Where("status = ?", status).Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var users []entities.User
	err := u.db.WithContext(c).
		Preload("Role").
		Preload("Provider").
		Preload("Provider.ProviderType").
		Preload("Consumer").
		Where("status = ?", status).
		Order("created_at DESC, id").
		Limit(limit).
		Offset(offset).
		Find(&users).Error
	return users, total, err
}

// UpdateUserStatus changes only the status column of a user.
func (u *userRepo) UpdateUserStatus(c context.Context, id string, status entities.UserStatus) error {
	res := u.db.WithContext(c).Model(&entities.User{}).Where("id = ?", id).Update("status", status)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return gorm.ErrRecordNotFound
	}
	return nil
}

func (u *userRepo) CreateProviderType(c context.Context, providerType *entities.ProviderType) error {
	return gorm.G[entities.ProviderType](u.db).Create(c, providerType)
}

func (u *userRepo) GetProviderTypes(c context.Context) ([]entities.ProviderType, error) {
	return gorm.G[entities.ProviderType](u.db).Order("id ASC").Find(c)
}

func (u *userRepo) GetProviderTypeById(c context.Context, id uint) (*entities.ProviderType, error) {
	pt, err := gorm.G[entities.ProviderType](u.db).Where("id = ?", id).First(c)
	if err != nil {
		return nil, err
	}
	return &pt, nil
}

func (u *userRepo) ProviderTypeExistsByNameOrSlug(c context.Context, name string, slug string) (bool, error) {
	var count int64
	err := u.db.WithContext(c).
		Model(&entities.ProviderType{}).
		Where("LOWER(name) = LOWER(?) OR LOWER(slug) = LOWER(?)", name, slug).
		Count(&count).Error
	return count > 0, err
}
