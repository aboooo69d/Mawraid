package valueHashing

import (
	"golang.org/x/crypto/bcrypt"
)

// HashValue generates a bcrypt hash for the given password using bcrypt.DefaultCost.
func HashValue(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// VerifyValue verifies if the given password matches the stored hash.
func VerifyValue(password, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password))
	return err == nil
}
