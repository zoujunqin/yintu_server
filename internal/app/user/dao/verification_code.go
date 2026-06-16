package dao

import (
	"context"
	"time"

	"gorm.io/gorm"

	"spring-slumber-server/internal/app/user/model"
)

// VerificationCodeDAO 封装 verification_code 表的访问。
type VerificationCodeDAO struct {
	db *gorm.DB
}

// NewVerificationCodeDAO 构造 VerificationCodeDAO。
func NewVerificationCodeDAO(db *gorm.DB) *VerificationCodeDAO {
	return &VerificationCodeDAO{db: db}
}

// Insert 写入一条验证码记录。
func (d *VerificationCodeDAO) Insert(ctx context.Context, vc *model.VerificationCode) error {
	return d.db.WithContext(ctx).Create(vc).Error
}

// LatestActive 查询指定手机号 + 场景的「最新一条未使用、未过期」验证码。
func (d *VerificationCodeDAO) LatestActive(ctx context.Context, phone, purpose string) (*model.VerificationCode, error) {
	var vc model.VerificationCode
	now := time.Now()
	err := d.db.WithContext(ctx).
		Where("phone_number = ? AND purpose = ? AND used = ? AND expire_at > ?", phone, purpose, false, now).
		Order("id DESC").
		First(&vc).Error
	if err != nil {
		return nil, err
	}
	return &vc, nil
}

// MarkUsed 将记录置为已使用。
func (d *VerificationCodeDAO) MarkUsed(ctx context.Context, id int64) error {
	return d.db.WithContext(ctx).Model(&model.VerificationCode{}).
		Where("id = ?", id).
		Update("used", true).Error
}
