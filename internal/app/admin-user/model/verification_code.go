package model

import "time"

// VerificationCode 短信验证码表。
//
// code_hash 存储 SHA256(phone_number + code + salt) 摘要，
// 明文验证码不入库；used=true 即视为已消费。
type VerificationCode struct {
	ID          int64     `gorm:"primaryKey;column:id"                                         json:"id"`
	PhoneNumber string    `gorm:"column:phone_number;size:20;index:idx_phone_purpose,priority:1" json:"phoneNumber"`
	CodeHash    string    `gorm:"column:code_hash;size:128"                                     json:"-"`
	Purpose     string    `gorm:"column:purpose;size:16;index:idx_phone_purpose,priority:2"      json:"purpose"`
	Used        bool      `gorm:"column:used;default:false"                                     json:"used"`
	ExpireAt    time.Time `gorm:"column:expire_at"                                              json:"expireAt"`
	CreatedAt   time.Time `gorm:"column:created_at;autoCreateTime"                              json:"createdAt"`
}

// TableName 指定 GORM 使用的表名。
func (VerificationCode) TableName() string { return "verification_code" }

// PurposeLogin 登录场景的验证码。
const PurposeLogin = "login"
