package service

import (
	"errors"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
	"github.com/yunnysunny/docker-image-proxy/internal/config"
)

type TokenService struct {
	log    *logrus.Logger
	config *config.Config
}
type Access struct {
	Type    string   `json:"type"`
	Actions []string `json:"actions"`
	Name    string   `json:"name"`
}
type Token struct {
	Aud    string   `json:"aud"`
	Iss    string   `json:"iss"`
	Sub    string   `json:"sub"`
	Jti    string   `json:"jti"`
	Exp    int64    `json:"exp"`
	Iat    int64    `json:"iat"`
	Nbf    int64    `json:"nbf"`
	Access []Access `json:"access"`
}

// GetAudience implements jwt.Claims.
func (t *Token) GetAudience() (jwt.ClaimStrings, error) {
	return jwt.ClaimStrings{t.Aud}, nil
}

// GetExpirationTime implements jwt.Claims.
func (t *Token) GetExpirationTime() (*jwt.NumericDate, error) {
	return &jwt.NumericDate{Time: time.Unix(t.Exp, 0)}, nil
}

// GetIssuedAt implements jwt.Claims.
func (t *Token) GetIssuedAt() (*jwt.NumericDate, error) {
	return &jwt.NumericDate{Time: time.Unix(t.Iat, 0)}, nil
}

// GetIssuer implements jwt.Claims.
func (t *Token) GetIssuer() (string, error) {
	return t.Iss, nil
}

// GetNotBefore implements jwt.Claims.
func (t *Token) GetNotBefore() (*jwt.NumericDate, error) {
	return &jwt.NumericDate{Time: time.Unix(t.Nbf, 0)}, nil
}

// GetSubject implements jwt.Claims.
func (t *Token) GetSubject() (string, error) {
	return t.Sub, nil
}

func NewTokenService(log *logrus.Logger, config *config.Config) *TokenService {
	return &TokenService{
		log:    log,
		config: config,
	}
}

/*
scope 格式举例:
- repository:library/ubuntu:pull
- repository:library/ubuntu:push
- repository:library/ubuntu:all
- repository:library/ubuntu:read
*/
func (s *TokenService) GetDockerRegistryToken(scope string) (string, error) {
	parts := strings.Split(scope, ":")
	typ := parts[0] // 修正变量名，避免与关键字冲突
	name := parts[1]
	action := parts[2]
	token := &Token{
		Aud: s.config.SelfRegistry,
		Iss: s.config.SelfRegistry,
		Sub: s.config.SelfRegistry,
		Jti: uuid.New().String(),
		Exp: time.Now().Add(time.Hour * 24).Unix(),
		Iat: time.Now().Unix(),
		Nbf: time.Now().Unix(),
		Access: []Access{
			{
				Type:    typ,
				Actions: []string{action},
				Name:    name,
			},
		},
	}
	tokenString, err := jwt.NewWithClaims(jwt.SigningMethodHS256, token).SignedString(
		[]byte(s.config.ServerSecret),
	)
	if err != nil {
		return "", err
	}
	return tokenString, nil
}

func (s *TokenService) GetToken(tokenString string) (*Token, error) {

	token, err := jwt.ParseWithClaims(tokenString, &Token{}, func(token *jwt.Token) (interface{}, error) {
		return []byte(s.config.ServerSecret), nil
	})
	if err != nil {
		return nil, err
	}
	if claims, ok := token.Claims.(*Token); ok && token.Valid {
		return claims, nil
	}
	return nil, errors.New("invalid token")
}

func (s *TokenService) GetUnverifiedToken(tokenString string) (*Token, error) {
	token, _,err := new (jwt.Parser).ParseUnverified(tokenString, &Token{})
	if err != nil {
		return nil, err
	}
	return token.Claims.(*Token), nil
}
