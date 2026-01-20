package utils

import (
	"strings"
)

func IntPtr(i int) *int {
	if i == 0 {
		return nil
	}
	return &i
}

func IsLockTimeout(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "Lock wait timeout") || strings.Contains(errStr, "deadlock") || strings.Contains(errStr, "try restarting transaction")
}

func MaskValue(s string) string {
	if len(s) <= 8 {
		return "****"
	}
	return s[:4] + "****" + s[len(s)-4:]
}
