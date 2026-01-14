package token

import (
	"errors"
	"fmt"
	"time"

	"github.com/code19m/errx"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

const (
	CodeExpiredToken = "EXPIRED_TOKEN"
	CodeInvalidToken = "INVALID_TOKEN"

	minSecretKeySize = 16
)

// JWTMaker is a JSON Web Token maker.
type JWTMaker struct {
	secretKey string
}

// NewJWTMaker creates a new JWTMaker.
func NewJWTMaker(secretKey string) (*JWTMaker, error) {
	if len(secretKey) < minSecretKeySize {
		return nil, errx.New(fmt.Sprintf("invalid key size: must be at least %d characters", minSecretKeySize))
	}
	return &JWTMaker{secretKey}, nil
}

func (maker *JWTMaker) CreateToken(
	sub string,
	duration time.Duration,
	customClaims map[string]any,
) (string, *Payload, error) {
	payload, err := NewPayload(sub, duration)
	if err != nil {
		return "", nil, errx.Wrap(err)
	}

	// Add custom claims to the payload
	for key, value := range customClaims {
		payload = payload.WithCustomClaim(key, value)
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)
	token, err := jwtToken.SignedString([]byte(maker.secretKey))
	if err != nil {
		return "", nil, errx.Wrap(err)
	}

	return token, payload, nil
}

func (maker *JWTMaker) VerifyToken(token string) (*Payload, error) {
	keyFunc := func(token *jwt.Token) (any, error) {
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, errx.New("unexpected signing method", errx.WithCode(CodeInvalidToken))
		}
		return []byte(maker.secretKey), nil
	}

	jwtToken, err := jwt.ParseWithClaims(token, &Payload{}, keyFunc)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, errx.New("token is expired", errx.WithCode(CodeExpiredToken))
		}
		return nil, errx.Wrap(err, errx.WithCode(CodeInvalidToken))
	}

	payload, ok := jwtToken.Claims.(*Payload)
	if !ok {
		return nil, errx.New("invalid token claims", errx.WithCode(CodeInvalidToken))
	}

	if !payload.Valid() {
		return nil, errx.New("token is invalid", errx.WithCode(CodeInvalidToken))
	}

	return payload, nil
}

// Payload contains the payload data of the token.
type Payload struct {
	// Standard claims
	ID        uuid.UUID        `json:"jti"`           // Unique identifier for the token
	Subject   string           `json:"sub"`           // Subject (UserID)
	IssuedAt  *jwt.NumericDate `json:"iat"`           // Issued At timestamp
	ExpiresAt *jwt.NumericDate `json:"exp"`           // Expiration timestamp
	NotBefore *jwt.NumericDate `json:"nbf,omitempty"` // Not valid before this time
	Issuer    string           `json:"iss,omitempty"` // Issuer
	Audience  []string         `json:"aud,omitempty"` // Audience

	// Additional custom claims
	CustomClaims map[string]any `json:"custom_claims,omitempty"`
}

// NewPayload creates a new token payload for a subject with the specified ID, type, role, and duration with custom claims.
func NewPayload(sub string, duration time.Duration) (*Payload, error) {
	tokenID, err := uuid.NewRandom()
	if err != nil {
		return nil, errx.Wrap(err)
	}

	now := time.Now()
	payload := &Payload{
		ID:           tokenID,
		Subject:      sub,
		IssuedAt:     jwt.NewNumericDate(now),
		ExpiresAt:    jwt.NewNumericDate(now.Add(duration)),
		NotBefore:    jwt.NewNumericDate(now), // NotBefore is set to now by default, can be adjusted later
		Issuer:       "",                      // Issuer is empty by default, can be set later
		Audience:     []string{},              // Audience is empty by default, can be set later
		CustomClaims: make(map[string]any),    // CustomClaims is empty by default, can be set later
	}
	return payload, nil
}

// WithNotBefore sets the NotBefore time for the token payload.
func (payload *Payload) WithNotBefore(notBefore time.Time) *Payload {
	payload.NotBefore = jwt.NewNumericDate(notBefore)
	return payload
}

func (payload *Payload) WithIssuer(issuer string) *Payload {
	payload.Issuer = issuer
	return payload
}

// WithAudience sets the Audience for the token payload.
func (payload *Payload) WithAudience(audience ...string) *Payload {
	if payload.Audience == nil {
		payload.Audience = make([]string, 0)
	}
	payload.Audience = append(payload.Audience, audience...)
	return payload
}

// WithCustomClaim adds a custom claim to the token payload.
func (payload *Payload) WithCustomClaim(key string, value any) *Payload {
	if payload.CustomClaims == nil {
		payload.CustomClaims = make(map[string]any)
	}
	payload.CustomClaims[key] = value
	return payload
}

// Valid checks if the token payload is valid.
func (payload *Payload) Valid() bool {
	now := time.Now()

	// Check if expired
	if payload.ExpiresAt != nil && now.After(payload.ExpiresAt.Time) {
		return false
	}

	// Check if before valid time
	if payload.NotBefore != nil && now.Before(payload.NotBefore.Time) {
		return false
	}

	return true
}

// GetExpirationTime returns the expiration time of the token payload as a jwt.NumericDate.
func (payload *Payload) GetExpirationTime() (*jwt.NumericDate, error) {
	return payload.ExpiresAt, nil
}

// GetIssuedAt returns the issued at time of the token payload as a jwt.NumericDate.
func (payload *Payload) GetIssuedAt() (*jwt.NumericDate, error) {
	return payload.IssuedAt, nil
}

// GetNotBefore returns the not before time of the token payload as a jwt.NumericDate.
func (payload *Payload) GetNotBefore() (*jwt.NumericDate, error) {
	return payload.NotBefore, nil
}

// GetIssuer returns the issuer of the token payload.
func (payload *Payload) GetIssuer() (string, error) {
	return payload.Issuer, nil
}

// GetSubject returns the subject of the token payload.
func (payload *Payload) GetSubject() (string, error) {
	return payload.Subject, nil
}

// GetAudience returns the audience of the token payload as a jwt.ClaimStrings.
func (payload *Payload) GetAudience() (jwt.ClaimStrings, error) {
	return payload.Audience, nil
}
