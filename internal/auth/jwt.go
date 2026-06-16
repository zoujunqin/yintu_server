// Package auth 提供 JWT 颁发与解析。
package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"spring-slumber-server/internal/config"
)

// Claims 业务字段，sub 写 uid。
type Claims struct {
	UID         int64  `json:"uid"`
	PhoneNumber string `json:"phoneNumber"`
	jwt.RegisteredClaims
}

// Issuer 负责签发 / 解析 JWT。
type Issuer struct {
	secret    []byte
	issuer    string
	expiresIn time.Duration
}

// NewIssuer 构造 Issuer。
func NewIssuer(cfg config.JWTConfig) *Issuer {
	return &Issuer{
		secret:    []byte(cfg.Secret),
		issuer:    cfg.Issuer,
		expiresIn: cfg.ExpiresIn,
	}
}

// Issue 签发 token。
func (i *Issuer) Issue(uid int64, phone string) (string, time.Time, error) {
	expiresAt := time.Now().Add(i.expiresIn)
	claims := Claims{
		UID:         uid,
		PhoneNumber: phone,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    i.issuer,
			Subject:   fmt.Sprintf("%d", uid),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(i.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign jwt: %w", err)
	}
	return signed, expiresAt, nil
}

// Parse 解析并校验 token。
func (i *Issuer) Parse(tokenStr string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return i.secret, nil
	})
	if err != nil {
		return nil, err
	}
	return claims, nil
}
