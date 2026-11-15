package auth

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Claims carried by the short-lived access token. Refresh tokens are opaque
// random strings tracked server-side in the auth_sessions table (revocable).
type Claims struct {
	UserID   uint   `json:"uid"`
	Role     string `json:"role"`
	TokenUse string `json:"use"` // "access" | "refresh_hint" (unused; refresh is opaque)
	jwt.RegisteredClaims
}

// JWTManager signs and verifies access tokens.
type JWTManager struct {
	secret    []byte
	accessTTL time.Duration
}

// NewJWTManager builds a manager with the given secret and access-token TTL.
func NewJWTManager(secret string, accessTTL time.Duration) *JWTManager {
	return &JWTManager{secret: []byte(secret), accessTTL: accessTTL}
}

// AccessTTL returns the configured access-token lifetime.
func (m *JWTManager) AccessTTL() time.Duration { return m.accessTTL }

// IssueAccessToken creates a signed HS256 access token for the user.
func (m *JWTManager) IssueAccessToken(userID uint, role string) (string, time.Time, error) {
	now := time.Now()
	exp := now.Add(m.accessTTL)
	claims := Claims{
		UserID:   userID,
		Role:     role,
		TokenUse: "access",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "mawarid",
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// VerifyAccessToken parses and validates an access token.
func (m *JWTManager) VerifyAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return nil, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return nil, errors.New("invalid token")
	}
	if claims.TokenUse != "access" {
		return nil, errors.New("wrong token use")
	}
	return claims, nil
}

// --- password-reset tokens ---------------------------------------------------
// A short-lived single-purpose JWT that authorizes exactly one password
// change after a verified OTP (SRS URS 1.3 scenario 6).

// IssuePasswordResetToken creates a signed reset token for userID.
func (m *JWTManager) IssuePasswordResetToken(userID uint, exp time.Time) (string, time.Time, error) {
	claims := Claims{
		UserID:   userID,
		TokenUse: "password_reset",
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    "mawarid",
			Subject:   fmt.Sprintf("%d", userID),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(exp),
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := t.SignedString(m.secret)
	if err != nil {
		return "", time.Time{}, err
	}
	return signed, exp, nil
}

// VerifyPasswordResetToken validates a reset token and returns its user id.
func (m *JWTManager) VerifyPasswordResetToken(tokenString string) (uint, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method: %v", t.Header["alg"])
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}), jwt.WithExpirationRequired())
	if err != nil {
		return 0, err
	}
	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid || claims.TokenUse != "password_reset" {
		return 0, errors.New("invalid reset token")
	}
	return claims.UserID, nil
}
