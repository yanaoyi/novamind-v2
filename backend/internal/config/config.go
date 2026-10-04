// Package config 负责加载运行时配置。
// 规则：配置只从环境变量读取；若存在 .env 则作为默认值来源（不覆盖已有环境变量）。
// .env 不提交 Git。
package config

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config 是后端全部运行时配置。
// 新增配置项时同步更新 .env.example。
type Config struct {
	AppEnv        string // development | production
	HTTPAddr      string // 监听地址，如 127.0.0.1:8080
	DatabaseURL   string // PostgreSQL 连接串
	RedisAddr     string
	RedisPassword string
	RedisDB       int
	LogLevel      string
	StorageDir    string
	UploadMaxMB   int
	// SecretKey 用于加密模型 API Key（环境变量 NOVAMIND_SECRET）。
	// 丢失它意味着已保存的模型密钥都解不开，只能重新填写。
	SecretKey string
	// AdminToken 是访问令牌（环境变量 ADMIN_TOKEN）。
	// 所有 /api/v1 业务接口都要求 `Authorization: Bearer <token>`；只放行健康检查与 API 文档。
	// 生产环境未配置时**拒绝启动**——不给"忘了配就等于全站裸奔"留后门。
	AdminToken string
	// AllowPrivateModelBase 允许把模型 api_base 指向内网/环回地址（环境变量 ALLOW_PRIVATE_MODEL_BASE）。
	// 默认关闭：这是 SSRF 收敛的关键（否则可把服务端当内网探针，甚至把入库的真实 Key 发到攻击者地址）。
	// 本地冒烟测试要用 127.0.0.1 的假模型服务，所以在 .env 里显式打开。
	AllowPrivateModelBase bool
}

// Load 读取 .env（若存在）并组装配置。
func Load(envFiles ...string) (*Config, error) {
	files := envFiles
	if len(files) == 0 {
		files = []string{".env"}
	}
	for _, f := range files {
		if err := loadDotEnv(f); err != nil {
			return nil, fmt.Errorf("加载 %s 失败: %w", f, err)
		}
	}

	redisDB, err := strconv.Atoi(getEnv("REDIS_DB", "0"))
	if err != nil {
		return nil, fmt.Errorf("REDIS_DB 必须是整数: %w", err)
	}

	cfg := &Config{
		AppEnv:        getEnv("APP_ENV", "development"),
		HTTPAddr:      getEnv("HTTP_ADDR", "127.0.0.1:8080"),
		DatabaseURL:   getEnv("DATABASE_URL", ""),
		RedisAddr:     getEnv("REDIS_ADDR", "127.0.0.1:6379"),
		RedisPassword: getEnv("REDIS_PASSWORD", ""),
		RedisDB:       redisDB,
		LogLevel:      getEnv("LOG_LEVEL", "info"),
		StorageDir:    getEnv("STORAGE_DIR", "./data/uploads"),
		SecretKey:     getEnv("NOVAMIND_SECRET", ""),
		AdminToken:    getEnv("ADMIN_TOKEN", ""),
	}
	cfg.AllowPrivateModelBase = getEnv("ALLOW_PRIVATE_MODEL_BASE", "") == "true"
	uploadMaxMB, err := strconv.Atoi(getEnv("UPLOAD_MAX_MB", "50"))
	if err != nil || uploadMaxMB <= 0 {
		return nil, fmt.Errorf("UPLOAD_MAX_MB 必须是正整数")
	}
	cfg.UploadMaxMB = uploadMaxMB
	if cfg.IsProduction() && cfg.AdminToken == "" {
		return nil, fmt.Errorf("生产环境必须配置 ADMIN_TOKEN（所有 /api/v1 接口的访问令牌）")
	}
	return cfg, nil
}

// UploadMaxBytes 返回上传大小上限（字节）。
func (c *Config) UploadMaxBytes() int64 { return int64(c.UploadMaxMB) * 1024 * 1024 }

// IsProduction 供日志与 Gin 模式判断使用。
func (c *Config) IsProduction() bool { return c.AppEnv == "production" }

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

// loadDotEnv 读取 KEY=VALUE 形式的文件；已存在的环境变量优先，不被覆盖。
// 文件不存在不算错误（生产环境只走环境变量）。
func loadDotEnv(path string) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			return err
		}
	}
	return scanner.Err()
}
