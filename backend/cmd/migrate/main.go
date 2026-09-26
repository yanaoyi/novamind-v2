// migrate 是数据库迁移执行器。
//
// 用法：
//
//	go run ./cmd/migrate up            # 执行全部未应用的迁移
//	go run ./cmd/migrate down          # 回滚 1 步
//	go run ./cmd/migrate down-all      # 全部回滚
//	go run ./cmd/migrate version       # 查看当前版本
//	go run ./cmd/migrate force <ver>   # 强制写入版本（脏状态救援，慎用）
//
// 连接串取自 DATABASE_URL（可写在 backend/.env）。
package main

import (
	"errors"
	"fmt"
	"os"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"

	"github.com/yanaoyi/novamindv2/backend/internal/config"
	"github.com/yanaoyi/novamindv2/backend/migrations"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "迁移失败:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	if cfg.DatabaseURL == "" {
		return errors.New("DATABASE_URL 未配置（可在 backend/.env 中设置）")
	}

	m, err := newMigrator(cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		srcErr, dbErr := m.Close()
		if srcErr != nil {
			fmt.Fprintln(os.Stderr, "关闭迁移源失败:", srcErr)
		}
		if dbErr != nil {
			fmt.Fprintln(os.Stderr, "关闭数据库失败:", dbErr)
		}
	}()

	cmd := "up"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "up":
		return report(m, m.Up())
	case "down":
		return report(m, m.Steps(-1))
	case "down-all":
		return report(m, m.Down())
	case "version":
		version, dirty, err := m.Version()
		if errors.Is(err, migrate.ErrNilVersion) {
			fmt.Println("当前版本: 无（尚未执行任何迁移）")
			return nil
		}
		if err != nil {
			return err
		}
		fmt.Printf("当前版本: %d（dirty=%v）\n", version, dirty)
		return nil
	case "force":
		if len(os.Args) < 3 {
			return errors.New("force 需要版本号参数，例如：force 1")
		}
		v, err := strconv.Atoi(os.Args[2])
		if err != nil {
			return fmt.Errorf("版本号必须是整数: %w", err)
		}
		return report(m, m.Force(v))
	default:
		return fmt.Errorf("未知命令 %q（可用：up / down / down-all / version / force）", cmd)
	}
}

func newMigrator(databaseURL string) (*migrate.Migrate, error) {
	src, err := iofs.New(migrations.FS, ".")
	if err != nil {
		return nil, fmt.Errorf("读取内嵌迁移失败: %w", err)
	}
	m, err := migrate.NewWithSourceInstance("iofs", src, databaseURL)
	if err != nil {
		return nil, fmt.Errorf("初始化迁移器失败: %w", err)
	}
	return m, nil
}

func report(m *migrate.Migrate, err error) error {
	if errors.Is(err, migrate.ErrNoChange) {
		fmt.Println("没有需要执行的迁移（数据库已是最新）")
		return printVersion(m)
	}
	if err != nil {
		return err
	}
	fmt.Println("迁移执行完成")
	return printVersion(m)
}

func printVersion(m *migrate.Migrate) error {
	version, dirty, err := m.Version()
	if errors.Is(err, migrate.ErrNilVersion) {
		// 回滚到空库时版本为 nil，这不是错误
		fmt.Println("当前版本: 无（数据库已回滚到空状态）")
		return nil
	}
	if err != nil {
		return err
	}
	fmt.Printf("当前版本: %d（dirty=%v）\n", version, dirty)
	return nil
}
