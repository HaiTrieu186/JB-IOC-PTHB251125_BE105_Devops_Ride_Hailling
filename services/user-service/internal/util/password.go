package util

import (
	"golang.org/x/crypto/bcrypt"
)

// DummyPasswordHash là chuỗi bcrypt hash hợp lệ cost 10 chuẩn 60 ký tự, dùng để chống timing attack
const DummyPasswordHash = "$2a$10$N9qo8uLOickgx2ZMRZoMyeIjZAgcfl7p92ldGxad68LJZdL17lhWy"

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}

func CheckPassword(hash string, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}

func CheckDummyPassword(password string) {
	_ = bcrypt.CompareHashAndPassword([]byte(DummyPasswordHash), []byte(password))
}
