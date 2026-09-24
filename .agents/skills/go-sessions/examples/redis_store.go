package examples

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisSessionStore محرك تخزين الجلسات الموزع في Redis مع فهرسة ثانوية
type RedisSessionStore struct {
	client redis.UniversalClient
	prefix string
}

func NewRedisSessionStore(client redis.UniversalClient, prefix string) *RedisSessionStore {
	if prefix == "" {
		prefix = "sess:"
	}
	return &RedisSessionStore{
		client: client,
		prefix: prefix,
	}
}

func (s *RedisSessionStore) sessionKey(tokenHash string) string {
	return s.prefix + tokenHash
}

func (s *RedisSessionStore) userKey(userID string) string {
	return "user_sessions:" + userID
}

// Get يسترجع بيانات الجلسة من Redis
func (s *RedisSessionStore) Get(ctx context.Context, tokenHash string) (*SessionData, error) {
	val, err := s.client.Get(ctx, s.sessionKey(tokenHash)).Bytes()
	if err != nil {
		if errors.Is(err, redis.Nil) {
			return nil, ErrSessionNotFound
		}
		return nil, fmt.Errorf("redis get error: %w", err)
	}

	var data SessionData
	if err := json.Unmarshal(val, &data); err != nil {
		return nil, fmt.Errorf("failed to unmarshal session: %w", err)
	}

	return &data, nil
}

// Set يحفظ بيانات الجلسة ويُحدث الفهرس الثانوي للمستخدم عبر خط أنابيب (Pipeline)
func (s *RedisSessionStore) Set(ctx context.Context, tokenHash string, data *SessionData, ttl time.Duration) error {
	payload, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	pipe := s.client.Pipeline()
	pipe.Set(ctx, s.sessionKey(tokenHash), payload, ttl)
	
	if data.UserID != "" {
		pipe.SAdd(ctx, s.userKey(data.UserID), tokenHash)
		// تعيين انتهاء صلاحية لمجموعة المستخدم مساوٍ للمهلة المطلقة
		pipe.Expire(ctx, s.userKey(data.UserID), 24*time.Hour)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis set pipeline failed: %w", err)
	}

	return nil
}

// Delete يحذف الجلسة ويزيل معرفها من مجموعة المستخدم
func (s *RedisSessionStore) Delete(ctx context.Context, tokenHash string) error {
	data, err := s.Get(ctx, tokenHash)
	if err != nil && !errors.Is(err, ErrSessionNotFound) {
		return err
	}

	pipe := s.client.Pipeline()
	pipe.Del(ctx, s.sessionKey(tokenHash))
	if data != nil && data.UserID != "" {
		pipe.SRem(ctx, s.userKey(data.UserID), tokenHash)
	}

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("redis delete pipeline failed: %w", err)
	}

	return nil
}

// DeleteByUserID يطرد كافة جلسات المستخدم من جميع الأجهزة
func (s *RedisSessionStore) DeleteByUserID(ctx context.Context, userID string) error {
	uKey := s.userKey(userID)
	hashes, err := s.client.SMembers(ctx, uKey).Result()
	if err != nil {
		return fmt.Errorf("failed to fetch user sessions: %w", err)
	}

	if len(hashes) == 0 {
		return nil
	}

	pipe := s.client.Pipeline()
	for _, h := range hashes {
		pipe.Del(ctx, s.sessionKey(h))
	}
	pipe.Del(ctx, uKey)

	_, err = pipe.Exec(ctx)
	if err != nil {
		return fmt.Errorf("failed to delete user sessions: %w", err)
	}

	return nil
}
