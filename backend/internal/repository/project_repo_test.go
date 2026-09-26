package repository

import (
	"context"
	"errors"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/yanaoyi/novamindv2/backend/internal/config"
	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/infra"
)

// newTestRepo 返回一个"每个测试独立事务、结束时回滚"的仓储，保证测试不污染开发库。
// 未配置 DATABASE_URL 或表不存在时跳过（保持 go test ./... 在无库环境下可用）。
func newTestRepo(t *testing.T) *ProjectRepo {
	t.Helper()

	if _, err := config.Load("../../.env", ".env"); err != nil {
		t.Fatalf("加载配置失败: %v", err)
	}
	dsn := os.Getenv("DATABASE_URL")
	if dsn == "" {
		t.Skip("未配置 DATABASE_URL，跳过 repository 集成测试")
	}

	pg, err := infra.NewPostgres(context.Background(), dsn, false)
	if err != nil {
		t.Skipf("PostgreSQL 不可用，跳过集成测试: %v", err)
	}
	t.Cleanup(func() { _ = pg.Close() })

	var hasTable bool
	if err := pg.DB.Raw("SELECT to_regclass('public.projects') IS NOT NULL").Scan(&hasTable).Error; err != nil {
		t.Fatalf("探测 projects 表失败: %v", err)
	}
	if !hasTable {
		t.Skip("projects 表不存在，请先执行：go run ./cmd/migrate up")
	}

	tx := pg.DB.Begin()
	if tx.Error != nil {
		t.Fatalf("开启事务失败: %v", tx.Error)
	}
	t.Cleanup(func() { _ = tx.Rollback() })
	return NewProjectRepo(tx)
}

func uniqueName(prefix string) string {
	return fmt.Sprintf("%s-%d", prefix, time.Now().UnixNano())
}

func TestProjectRepoCreateAndGet(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	p := &domain.Project{
		Name:        uniqueName("集成测试-二创"),
		Description: "由 repository 测试创建",
		Type:        domain.ProjectTypeCreative,
		Status:      domain.ProjectStatusDraft,
	}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if p.ID == "" {
		t.Fatal("创建后 ID 为空")
	}
	if _, err := uuid.Parse(p.ID); err != nil {
		t.Fatalf("ID 不是合法 UUID: %v", err)
	}
	if p.CreatedAt.IsZero() || p.UpdatedAt.IsZero() {
		t.Fatal("时间戳未回填")
	}

	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("查询失败: %v", err)
	}
	if got.Name != p.Name || got.Type != domain.ProjectTypeCreative || got.Status != domain.ProjectStatusDraft {
		t.Fatalf("查询结果不一致: %+v", got)
	}
}

func TestProjectRepoGetNotFound(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	if _, err := repo.GetByID(ctx, uuid.NewString()); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Errorf("不存在的 ID 应返回 ErrProjectNotFound，实际 %v", err)
	}
	if _, err := repo.GetByID(ctx, "not-a-uuid"); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Errorf("非法 UUID 应返回 ErrProjectNotFound，实际 %v", err)
	}
}

func TestProjectRepoUpdate(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	p := &domain.Project{Name: uniqueName("更新前"), Type: domain.ProjectTypeOriginal}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	p.Name = uniqueName("更新后")
	p.Description = "已更新"
	p.Status = domain.ProjectStatusArchived
	if err := repo.Update(ctx, p); err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	got, err := repo.GetByID(ctx, p.ID)
	if err != nil {
		t.Fatalf("更新后查询失败: %v", err)
	}
	if got.Name != p.Name || got.Description != "已更新" || got.Status != domain.ProjectStatusArchived {
		t.Fatalf("更新未生效: %+v", got)
	}

	// 不存在的记录
	ghost := &domain.Project{ID: uuid.NewString(), Name: "幽灵", Type: domain.ProjectTypeOriginal}
	if err := repo.Update(ctx, ghost); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Errorf("更新不存在的记录应返回 ErrProjectNotFound，实际 %v", err)
	}
}

func TestProjectRepoSoftDelete(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	p := &domain.Project{Name: uniqueName("待删除"), Type: domain.ProjectTypeOriginal}
	if err := repo.Create(ctx, p); err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	if err := repo.SoftDelete(ctx, p.ID); err != nil {
		t.Fatalf("软删除失败: %v", err)
	}
	if _, err := repo.GetByID(ctx, p.ID); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Errorf("软删除后不应查到，实际 %v", err)
	}

	// 物理行必须还在，且 deleted_at 已写入
	var rowCount int64
	if err := repo.db.Raw("SELECT count(*) FROM projects WHERE id = ?", p.ID).Scan(&rowCount).Error; err != nil {
		t.Fatalf("原始查询失败: %v", err)
	}
	if rowCount != 1 {
		t.Fatalf("软删除应是逻辑删除，物理行数为 %d", rowCount)
	}
	var deletedAt *time.Time
	if err := repo.db.Raw("SELECT deleted_at FROM projects WHERE id = ?", p.ID).Scan(&deletedAt).Error; err != nil {
		t.Fatalf("读取 deleted_at 失败: %v", err)
	}
	if deletedAt == nil {
		t.Fatal("deleted_at 未写入")
	}

	// 重复删除应报不存在
	if err := repo.SoftDelete(ctx, p.ID); !errors.Is(err, domain.ErrProjectNotFound) {
		t.Errorf("重复删除应返回 ErrProjectNotFound，实际 %v", err)
	}
}

func TestProjectRepoListFilterAndPaginate(t *testing.T) {
	repo := newTestRepo(t)
	ctx := context.Background()

	tag := uniqueName("列表")
	originalType := domain.ProjectTypeOriginal
	types := []domain.ProjectType{originalType, originalType, domain.ProjectTypeCreative}
	for i, ty := range types {
		p := &domain.Project{Name: fmt.Sprintf("%s-%d", tag, i), Type: ty}
		if err := repo.Create(ctx, p); err != nil {
			t.Fatalf("创建第 %d 条失败: %v", i, err)
		}
	}

	items, total, err := repo.List(ctx, ProjectFilter{Keyword: tag, Page: 1, PageSize: 2})
	if err != nil {
		t.Fatalf("列表查询失败: %v", err)
	}
	if total != 3 {
		t.Fatalf("总数应为 3，实际 %d", total)
	}
	if len(items) != 2 {
		t.Fatalf("首页应返回 2 条，实际 %d", len(items))
	}

	filtered, filteredTotal, err := repo.List(ctx, ProjectFilter{Keyword: tag, Type: &originalType})
	if err != nil {
		t.Fatalf("按类型过滤失败: %v", err)
	}
	if filteredTotal != 2 || len(filtered) != 2 {
		t.Fatalf("ORIGINAL 应命中 2 条，实际 total=%d len=%d", filteredTotal, len(filtered))
	}
	for _, it := range filtered {
		if it.Type != domain.ProjectTypeOriginal {
			t.Fatalf("过滤结果混入非 ORIGINAL: %+v", it)
		}
	}

	// 分页边界：page_size 超上限应被钳制
	f := ProjectFilter{Keyword: tag, Page: 0, PageSize: 1000}
	offset, limit := f.Normalize()
	if offset != 0 || limit != maxPageSize {
		t.Fatalf("分页参数未钳制: offset=%d limit=%d", offset, limit)
	}

}
