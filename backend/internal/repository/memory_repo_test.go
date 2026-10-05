package repository

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// 记忆仓储的集成测试（真库，跑在事务里、结束回滚）。
//
// 造数据用最小 SQL：memory_facts 的 creative_work_id 是外键，
// 而 creative_works.original_work_id 也是 NOT NULL —— 必须真造一条
// 「工程 → 原著 → 二创」的链路，不能拿随机 UUID 糊过去。
func newMemoryRepoForTest(t *testing.T) (*MemoryRepo, string, string, string) {
	t.Helper()
	base := newTestRepo(t)
	ctx := context.Background()
	userID, err := NewUserRepo(base.db).DefaultUserID(ctx)
	if err != nil {
		t.Fatalf("取默认用户失败: %v", err)
	}
	projectID, originalID, creativeID := uuid.NewString(), uuid.NewString(), uuid.NewString()
	exec := func(sql string, args ...any) {
		t.Helper()
		if err := base.db.WithContext(ctx).Exec(sql, args...).Error; err != nil {
			t.Fatalf("造测试数据失败: %v", err)
		}
	}
	exec(`insert into projects (id, name, type) values (?, '记忆测试-原著', 'ORIGINAL')`, projectID)
	exec(`insert into original_works (id, project_id, title) values (?, ?, '记忆测试原著')`, originalID, projectID)
	creativeProjectID := uuid.NewString()
	exec(`insert into projects (id, name, type) values (?, '记忆测试-二创', 'CREATIVE')`, creativeProjectID)
	exec(`insert into creative_works (id, project_id, original_work_id, title) values (?, ?, ?, '记忆测试同人')`,
		creativeID, creativeProjectID, originalID)

	return NewMemoryRepo(base.db), userID, creativeID, creativeProjectID
}

func newChapterForTest(t *testing.T, repo *MemoryRepo, workID string, no int) string {
	t.Helper()
	id := uuid.NewString()
	if err := repo.db.Exec(
		`insert into creative_chapters (id, creative_work_id, chapter_no, title, content) values (?, ?, ?, ?, '')`,
		id, workID, no, "测试章",
	).Error; err != nil {
		t.Fatalf("造章节失败: %v", err)
	}
	return id
}

func TestMemoryFactsSupersedeAndKeepHistory(t *testing.T) {
	ctx := context.Background()
	repo, userID, workID, _ := newMemoryRepoForTest(t)
	chapterID := newChapterForTest(t, repo, workID, 1)

	outcome, err := repo.CreateFacts(ctx, userID, workID, []FactWrite{
		{Kind: string(domain.FactCharacterState), Subject: "沈砚", Fact: "左臂受伤，无法用剑", ChapterID: &chapterID},
		{Kind: string(domain.FactItem), Subject: "青铜钥匙", Fact: "藏在祖宅第三块砖下", ChapterID: &chapterID},
	})
	if err != nil {
		t.Fatalf("写入事实失败: %v", err)
	}
	if len(outcome.Created) != 2 || len(outcome.Superseded) != 0 {
		t.Fatalf("首次写入应为 2 新增 0 替代，实际 %d/%d", len(outcome.Created), len(outcome.Superseded))
	}
	if outcome.Created[0].ID == "" {
		t.Error("新增事实必须带回 ID（调用方要用它写检索索引）")
	}

	// 同一 subject+kind 的新状态：旧事实被替代（不是删除）
	outcome, err = repo.CreateFacts(ctx, userID, workID, []FactWrite{
		{Kind: string(domain.FactCharacterState), Subject: "沈砚", Fact: "左臂已痊愈"},
	})
	if err != nil {
		t.Fatalf("写入新状态失败: %v", err)
	}
	if len(outcome.Created) != 1 || len(outcome.Superseded) != 1 {
		t.Fatalf("应新增 1 条并替代 1 条，实际 %d/%d", len(outcome.Created), len(outcome.Superseded))
	}

	active, err := repo.ListFacts(ctx, workID, true)
	if err != nil {
		t.Fatalf("查询有效事实失败: %v", err)
	}
	if len(active) != 2 {
		t.Fatalf("有效事实应为 2 条（受伤的那条已被替代），实际 %d", len(active))
	}
	for _, f := range active {
		if f.Fact == "左臂受伤，无法用剑" {
			t.Error("被替代的事实不该出现在 onlyActive 结果里")
		}
	}

	all, err := repo.ListFacts(ctx, workID, false)
	if err != nil {
		t.Fatalf("查询全部事实失败: %v", err)
	}
	if len(all) != 3 {
		t.Fatalf("全部事实应为 3 条（含被替代的，可追溯），实际 %d", len(all))
	}
	var supersededCount int
	for _, f := range all {
		if f.SupersededBy != nil {
			supersededCount++
		}
	}
	if supersededCount != 1 {
		t.Errorf("应恰好有 1 条被标记替代，实际 %d", supersededCount)
	}
}

func TestMemoryFactsSkipsIdenticalFact(t *testing.T) {
	ctx := context.Background()
	repo, userID, workID, _ := newMemoryRepoForTest(t)
	fact := FactWrite{Kind: string(domain.FactPlot), Subject: "主线", Fact: "沈家账册被改"}

	if _, err := repo.CreateFacts(ctx, userID, workID, []FactWrite{fact}); err != nil {
		t.Fatalf("首次写入失败: %v", err)
	}
	outcome, err := repo.CreateFacts(ctx, userID, workID, []FactWrite{fact})
	if err != nil {
		t.Fatalf("重复写入失败: %v", err)
	}
	if len(outcome.Created) != 0 || len(outcome.Superseded) != 0 {
		t.Errorf("重复抽取不该长出新行或替代旧行，实际 %d/%d", len(outcome.Created), len(outcome.Superseded))
	}
}

func TestMemoryFactsRejectsInvalidInput(t *testing.T) {
	ctx := context.Background()
	repo, userID, workID, _ := newMemoryRepoForTest(t)

	if _, err := repo.CreateFacts(ctx, userID, workID, []FactWrite{
		{Kind: "mood", Subject: "沈砚", Fact: "心情不错"},
	}); !errors.Is(err, domain.ErrFactKindInvalid) {
		t.Errorf("非法 kind 应被拒绝，实际 %v", err)
	}
	if _, err := repo.CreateFacts(ctx, userID, workID, []FactWrite{
		{Kind: string(domain.FactPlot), Subject: "  ", Fact: "有内容"},
	}); !errors.Is(err, domain.ErrFactSubjectEmpty) {
		t.Errorf("空 subject 应被拒绝，实际 %v", err)
	}
	// 事务性：一条非法就该整批不落库
	if all, err := repo.ListFacts(ctx, workID, false); err != nil || len(all) != 0 {
		t.Errorf("非法批次不该写入任何行，实际 %d 行 err=%v", len(all), err)
	}
}

func TestChapterSummaryUpsert(t *testing.T) {
	ctx := context.Background()
	repo, userID, workID, _ := newMemoryRepoForTest(t)
	chapterID := newChapterForTest(t, repo, workID, 1)

	if err := repo.UpsertSummary(ctx, userID, chapterID, "沈砚回到老宅，发现账册被改。"); err != nil {
		t.Fatalf("写摘要失败: %v", err)
	}
	if err := repo.UpsertSummary(ctx, userID, chapterID, "沈砚回到老宅，发现账册被改，并取走了青铜钥匙。"); err != nil {
		t.Fatalf("覆盖摘要失败: %v", err)
	}
	got, err := repo.GetSummary(ctx, chapterID)
	if err != nil {
		t.Fatalf("取摘要失败: %v", err)
	}
	if !strings.Contains(got.Summary, "青铜钥匙") {
		t.Errorf("摘要应被覆盖为最新内容（含新出现的细节），实际 %q", got.Summary)
	}
	if _, err := repo.GetSummary(ctx, uuid.NewString()); !errors.Is(err, domain.ErrChapterSummaryNotFound) {
		t.Errorf("不存在的章节应返回 ErrChapterSummaryNotFound，实际 %v", err)
	}
}
