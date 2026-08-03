package crypto

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"math/big"
)

// SHA256Hash returns the hex-encoded SHA-256 hash of the input.
func SHA256Hash(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// GenerateOTP generates a random numeric OTP of the specified length.
func GenerateOTP(length int) (string, error) {
	if length < 4 || length > 10 {
		return "", fmt.Errorf("OTP length must be between 4 and 10")
	}

	max := new(big.Int)
	max.Exp(big.NewInt(10), big.NewInt(int64(length)), nil)

	n, err := rand.Int(rand.Reader, max)
	if err != nil {
		return "", fmt.Errorf("generating OTP: %w", err)
	}

	return fmt.Sprintf("%0*d", length, n), nil
}

// GenerateRefreshToken generates a cryptographically secure random token.
func GenerateRefreshToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", fmt.Errorf("generating refresh token: %w", err)
	}
	return hex.EncodeToString(bytes), nil
}
