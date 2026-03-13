package auth

import (
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	jwt "github.com/golang-jwt/jwt/v5"
)

var (
	privateKeyCache sync.Map
	publicKeyCache  sync.Map
)

// SignUserToken creates an RS256 JWT for the supplied user.
func SignUserToken(privateKeyPath string, user User, ttl time.Duration) (string, error) {
	key, err := loadPrivateKey(privateKeyPath)
	if err != nil {
		return "", fmt.Errorf("load private key: %w", err)
	}

	now := time.Now().UTC()
	claims := Claims{
		UserID:         user.UserID,
		OrgID:          user.OrgID,
		Role:           user.Role,
		Email:          user.Email,
		GitHubUsername: user.GitHubUsername,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
			Subject:   user.UserID,
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signedToken, err := token.SignedString(key)
	if err != nil {
		return "", fmt.Errorf("sign jwt: %w", err)
	}

	return signedToken, nil
}

// ParseUserToken validates an RS256 JWT and returns the claims-backed user.
func ParseUserToken(publicKeyPath, rawToken string) (*User, error) {
	key, err := loadPublicKey(publicKeyPath)
	if err != nil {
		return nil, fmt.Errorf("load public key: %w", err)
	}

	claims := Claims{}
	parsedToken, err := jwt.ParseWithClaims(rawToken, &claims, func(token *jwt.Token) (interface{}, error) {
		if token.Method != jwt.SigningMethodRS256 {
			return nil, fmt.Errorf("unexpected signing method %q", token.Method.Alg())
		}
		return key, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse jwt: %w", err)
	}
	if !parsedToken.Valid {
		return nil, errors.New("jwt is invalid")
	}
	if strings.TrimSpace(claims.UserID) == "" {
		return nil, errors.New("jwt missing user_id")
	}

	return claims.ToUser(), nil
}

func loadPrivateKey(path string) (*rsa.PrivateKey, error) {
	if cached, ok := privateKeyCache.Load(path); ok {
		return cached.(*rsa.PrivateKey), nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, errors.New("private key PEM block missing")
	}

	privateKey, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		parsedKey, parseErr := x509.ParsePKCS8PrivateKey(block.Bytes)
		if parseErr != nil {
			return nil, fmt.Errorf("parse private key: %w", err)
		}
		var ok bool
		privateKey, ok = parsedKey.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("private key is not RSA")
		}
	}

	privateKeyCache.Store(path, privateKey)
	return privateKey, nil
}

func loadPublicKey(path string) (*rsa.PublicKey, error) {
	if cached, ok := publicKeyCache.Load(path); ok {
		return cached.(*rsa.PublicKey), nil
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	block, _ := pem.Decode(contents)
	if block == nil {
		return nil, errors.New("public key PEM block missing")
	}

	if publicKey, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		rsaPublicKey, ok := publicKey.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("public key is not RSA")
		}
		publicKeyCache.Store(path, rsaPublicKey)
		return rsaPublicKey, nil
	}

	certificate, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse public key: %w", err)
	}

	rsaPublicKey, ok := certificate.PublicKey.(*rsa.PublicKey)
	if !ok {
		return nil, errors.New("certificate public key is not RSA")
	}

	publicKeyCache.Store(path, rsaPublicKey)
	return rsaPublicKey, nil
}
