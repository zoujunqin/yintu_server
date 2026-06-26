// Package dao 负责 user feature 的数据访问。
package dao

import (
	"context"
	"errors"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"spring-slumber-server/internal/app/admin-user/model"
)

// ErrUserNotFound 用户不存在。
var ErrUserNotFound = errors.New("user not found")

// UserDAO 封装 user 表的查询/写入。
type UserDAO struct {
	db *gorm.DB
}

// NewUserDAO 构造 UserDAO。
func NewUserDAO(db *gorm.DB) *UserDAO {
	return &UserDAO{db: db}
}

// GetByPhone 根据手机号查询用户。
func (d *UserDAO) GetByPhone(ctx context.Context, phone string) (*model.User, error) {
	var u model.User
	err := d.db.WithContext(ctx).Where("phone_number = ?", phone).First(&u).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrUserNotFound
		}
		return nil, err
	}
	return &u, nil
}

// Create 创建用户，若 phone_number 冲突则使用 ON CONFLICT DO NOTHING 并回读。
func (d *UserDAO) Create(ctx context.Context, u *model.User) (*model.User, error) {
	if err := d.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(u).Error; err != nil {
		return nil, err
	}
	return d.GetByPhone(ctx, u.PhoneNumber)
}

// UpdateLoginMeta 刷新最后登录时间并清空锁定。
func (d *UserDAO) UpdateLoginMeta(ctx context.Context, id int64) error {
	now := gorm.Expr("NOW()")
	return d.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"last_login_at": now,
			"status":        model.UserStatusActive,
			"lock_time":     nil,
		}).Error
}

// Lock 锁定用户到指定时间。
func (d *UserDAO) Lock(ctx context.Context, id int64, until interface{}) error {
	return d.db.WithContext(ctx).Model(&model.User{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"status":    model.UserStatusLocked,
			"lock_time": until,
		}).Error
}
