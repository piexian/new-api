package common

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestValidateLoginPassword(t *testing.T) {
	for _, password := range []string{
		"Ab1!cdef",
		"Ab1!" + strings.Repeat("x", 68), // 恰好 72 字节
		"Ab1!" + strings.Repeat("中", 22) + "xy",
		" Ab1!xyz ", // 允许空格（不计入符号类别）
	} {
		require.NoError(t, ValidateLoginPassword(password), password)
	}
	for _, password := range []string{
		"",
		"Ab1!xyz",                        // 不足 8 字符
		"abcdefgh1!",                     // 无大写
		"ABCDEFGH1!",                     // 无小写
		"Abcdefgh!",                      // 无数字
		"Abcdefgh1",                      // 无符号
		"Abcdefg1中",                      // 中文不计符号
		"Ab1!" + strings.Repeat("x", 69), // 超过 72 字节
		"Ab1!" + strings.Repeat("中", 23),
		"Ab1!\xff\xfe00aa", // 非法 UTF-8
	} {
		require.ErrorIs(t, ValidateLoginPassword(password), ErrPasswordPolicy, password)
	}
}

func TestGenerateLoginPassword(t *testing.T) {
	for i := 0; i < 100; i++ {
		password, err := GenerateLoginPassword()
		require.NoError(t, err)
		require.NoError(t, ValidateLoginPassword(password))
		require.Len(t, password, 16)
	}
}
