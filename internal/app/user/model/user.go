// Package model 定义 user feature 的数据库实体。
package model

import "time"

// UserStatus 账号状态。
type UserStatus int8

const (
	// UserStatusActive 正常。
	UserStatusActive UserStatus = 0
	// UserStatusLocked 锁定中。
	UserStatusLocked UserStatus = 1
)

// User 用户表。
type User struct {
	ID          int64      `gorm:"primaryKey;column:id"                       json:"id"`
	PhoneNumber string     `gorm:"column:phone_number;size:20;uniqueIndex"    json:"phoneNumber"`
	Nickname    string     `gorm:"column:nickname;size:64"                    json:"nickname"`
	Avatar      string     `gorm:"column:avatar;size:255"                     json:"avatar"`
	Status      UserStatus `gorm:"column:status;default:0"                    json:"status"`
	LockTime    *time.Time `gorm:"column:lock_time"                           json:"lockTime,omitempty"`
	LastLoginAt *time.Time `gorm:"column:last_login_at"                       json:"lastLoginAt,omitempty"`
	CreatedAt   time.Time  `gorm:"column:created_at;autoCreateTime"          json:"createdAt"`
	UpdatedAt   time.Time  `gorm:"column:updated_at;autoUpdateTime"          json:"updatedAt"`
}

// TableName 指定 GORM 使用的表名。
func (User) TableName() string { return "user" }
