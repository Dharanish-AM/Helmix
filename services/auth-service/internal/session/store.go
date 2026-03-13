package session

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	sharedauth "github.com/your-org/helmix/libs/auth"
)

type sessionPayload struct {
	User sharedauth.User `json:"user"`
}

// Store manages refresh-token sessions in Redis.
type Store struct {
	client *redis.Client
	ttl    time.Duration
}

// New returns a refresh-token Redis store.
func New(redisURL string, ttl time.Duration) (*Store, error) {
	options, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, fmt.Errorf("parse redis url: %w", err)
	}
	client := redis.NewClient(options)
	if err := client.Ping(context.Background()).Err(); err != nil {
		return nil, fmt.Errorf("ping redis: %w", err)
	}
	return &Store{client: client, ttl: ttl}, nil
}

// Close closes the Redis client.
func (s *Store) Close() error {
	if s == nil || s.client == nil {
		return nil
	}
	if err := s.client.Close(); err != nil {
		return fmt.Errorf("close redis: %w", err)
	}
	return nil
}

// Create stores a refresh token for the supplied user.
func (s *Store) Create(ctx context.Context, user sharedauth.User) (string, error) {
	token, err := randomToken()
	if err != nil {
		return "", fmt.Errorf("generate refresh token: %w", err)
	}
	payload, err := json.Marshal(sessionPayload{User: user})
	if err != nil {
		return "", fmt.Errorf("marshal refresh token payload: %w", err)
	}
	if err := s.client.Set(ctx, key(token), payload, s.ttl).Err(); err != nil {
		return "", fmt.Errorf("store refresh token: %w", err)
	}
	return token, nil
}

// Rotate replaces an existing refresh token with a new one.
func (s *Store) Rotate(ctx context.Context, currentToken string) (sharedauth.User, string, error) {
	user, err := s.Get(ctx, currentToken)
	if err != nil {
		return sharedauth.User{}, "", fmt.Errorf("load refresh token: %w", err)
	}
	if err := s.Delete(ctx, currentToken); err != nil {
		return sharedauth.User{}, "", fmt.Errorf("delete old refresh token: %w", err)
	}
	newToken, err := s.Create(ctx, user)
	if err != nil {
		return sharedauth.User{}, "", fmt.Errorf("create rotated refresh token: %w", err)
	}
	return user, newToken, nil
}

// Delete removes a refresh token from Redis.
func (s *Store) Delete(ctx context.Context, token string) error {
	if token == "" {
		return nil
	}
	if err := s.client.Del(ctx, key(token)).Err(); err != nil {
		return fmt.Errorf("delete refresh token: %w", err)
	}
	return nil
}

// Get loads the user session bound to a refresh token.
func (s *Store) Get(ctx context.Context, token string) (sharedauth.User, error) {
	rawPayload, err := s.client.Get(ctx, key(token)).Bytes()
	if err != nil {
		return sharedauth.User{}, fmt.Errorf("get refresh token: %w", err)
	}
	var payload sessionPayload
	if err := json.Unmarshal(rawPayload, &payload); err != nil {
		return sharedauth.User{}, fmt.Errorf("unmarshal refresh token payload: %w", err)
	}
	return payload.User, nil
}

func key(token string) string {
	return "refresh:" + token
}

func randomToken() (string, error) {
	buffer := make([]byte, 32)
	if _, err := rand.Read(buffer); err != nil {
		return "", fmt.Errorf("read random bytes: %w", err)
	}
	return hex.EncodeToString(buffer), nil
}
