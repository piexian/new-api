package common

import (
	"crypto/rand"
	"errors"
	"math/big"
	"unicode/utf8"
)

// PasswordMaxBytes 对齐 bcrypt 的 72 字节输入上限。
const PasswordMaxBytes = 72

var ErrPasswordPolicy = errors.New("user.password_policy")

// ValidateLoginPassword 只校验新设置的登录密码，不用于已有凭据校验或
// Password2Hash 处理的其他值（如 2FA 验证码）。
func ValidateLoginPassword(password string) error {
	if !utf8.ValidString(password) || utf8.RuneCountInString(password) < 8 || len(password) > PasswordMaxBytes {
		return ErrPasswordPolicy
	}
	var digit, upper, lower, symbol bool
	for _, ch := range password {
		switch {
		case ch >= '0' && ch <= '9':
			digit = true
		case ch >= 'A' && ch <= 'Z':
			upper = true
		case ch >= 'a' && ch <= 'z':
			lower = true
		case ch >= '!' && ch <= '/' || ch >= ':' && ch <= '@' || ch >= '[' && ch <= '`' || ch >= '{' && ch <= '~':
			symbol = true
		}
	}
	if !digit || !upper || !lower || !symbol {
		return ErrPasswordPolicy
	}
	return nil
}

// GenerateLoginPassword 以拒绝采样保证生成的密码均匀分布且满足策略。
func GenerateLoginPassword() (string, error) {
	for {
		password := make([]byte, 16)
		for i := range password {
			n, err := rand.Int(rand.Reader, big.NewInt(94))
			if err != nil {
				return "", err
			}
			password[i] = byte(n.Int64()) + '!'
		}
		if ValidateLoginPassword(string(password)) == nil {
			return string(password), nil
		}
	}
}
