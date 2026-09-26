package infra

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// Redis 持有 Redis 客户端（Phase 1 只做连接与健康检查，
// Phase 3 起作为任务队列与短期记忆的存储）。
type Redis struct {
	Client *redis.Client
}

// NewRedis 建立连接并探测连通性（最多重试 3 次）。
func NewRedis(ctx context.Context, addr, password string, db int) (*Redis, error) {
	if addr == "" {
		return nil, errors.New("REDIS_ADDR 未配置")
	}
	client := redis.NewClient(&redis.Options{
		Addr:     addr,
		Password: password,
		DB:       db,
	})

	var lastErr error
	for i := 1; i <= 3; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = client.Ping(pingCtx).Err()
		cancel()
		if lastErr == nil {
			return &Redis{Client: client}, nil
		}
		if i < 3 {
			select {
			case <-ctx.Done():
				_ = client.Close()
				return nil, ctx.Err()
			case <-time.After(time.Duration(i) * 500 * time.Millisecond):
			}
		}
	}
	_ = client.Close()
	return nil, fmt.Errorf("Redis 连接不可用: %w", lastErr)
}

// Health 供 /api/v1/health 使用。
func (r *Redis) Health(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return r.Client.Ping(pingCtx).Err()
}

// Close 释放连接。
func (r *Redis) Close() error {
	if r == nil || r.Client == nil {
		return nil
	}
	return r.Client.Close()
}
