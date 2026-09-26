// Package infra 负责与外部基础设施（数据库、缓存、队列）建立连接。
// 约束：本包只做"连接与健康"，不写业务逻辑；业务通过 repository 使用连接。
package infra

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// Postgres 持有 PostgreSQL 连接池（GORM + 底层 *sql.DB）。
type Postgres struct {
	DB    *gorm.DB
	sqlDB *sql.DB
}

// NewPostgres 建立连接并做一次连通性探测（最多重试 3 次）。
func NewPostgres(ctx context.Context, dsn string, debug bool) (*Postgres, error) {
	if dsn == "" {
		return nil, errors.New("DATABASE_URL 未配置")
	}

	logLevel := gormlogger.Warn
	if debug {
		logLevel = gormlogger.Info
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormlogger.Default.LogMode(logLevel),
	})
	if err != nil {
		return nil, fmt.Errorf("打开 PostgreSQL 连接失败: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("获取底层连接池失败: %w", err)
	}
	sqlDB.SetMaxOpenConns(20)
	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetConnMaxIdleTime(10 * time.Minute)
	sqlDB.SetConnMaxLifetime(time.Hour)

	if err := pingWithRetry(ctx, sqlDB, 3); err != nil {
		_ = sqlDB.Close()
		return nil, fmt.Errorf("PostgreSQL 连接不可用: %w", err)
	}
	return &Postgres{DB: db, sqlDB: sqlDB}, nil
}

// Health 供 /api/v1/health 使用。
func (p *Postgres) Health(ctx context.Context) error {
	pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	return p.sqlDB.PingContext(pingCtx)
}

// Close 关闭连接池。
func (p *Postgres) Close() error {
	if p == nil || p.sqlDB == nil {
		return nil
	}
	return p.sqlDB.Close()
}

func pingWithRetry(ctx context.Context, db *sql.DB, attempts int) error {
	var lastErr error
	for i := 1; i <= attempts; i++ {
		pingCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		lastErr = db.PingContext(pingCtx)
		cancel()
		if lastErr == nil {
			return nil
		}
		if i < attempts {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(time.Duration(i) * 500 * time.Millisecond):
			}
		}
	}
	return lastErr
}
