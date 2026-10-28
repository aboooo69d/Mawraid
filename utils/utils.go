package utils

import (
	"crypto/rand"
	"math/big"
)

func GenerateSecureToken() (string, error) {
	const charset = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	token := make([]byte, 10)
	max := big.NewInt(int64(len(charset)))

	for i := range token {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			return "", err
		}

		token[i] = charset[n.Int64()]
	}

	return string(token), nil
}
