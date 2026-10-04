package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// VersionService 负责「人物 / 世界观 / 大纲」的状态快照与恢复（规格书 §59）。
//
// 语义（刻意保守）：
//   - 快照 = 某一刻该实体的完整状态；每次改动完成后再存一份，内容与上一版相同则跳过（不产生噪声版本）；
//   - 恢复 = 把快照里的字段写回去，并且**先把恢复前的内容也存一版**（和章节版本一致，避免误操作丢东西）；
//   - 恢复不回滚「删除」：比如快照之后新建的人物/规则不会因此消失，只回填快照里记录的实体字段。
type VersionService struct {
	repo     *repository.EntityVersionRepo
	creative *CreativeService
	writing  *WritingService
	// outlineTree 把大纲树（§27）的快照/恢复委托给大纲服务——写入路径只有一条，不给版本服务开后门。
	outlineTree OutlineTreeVersioner
}

// NewVersionService 构建服务。
func NewVersionService(repo *repository.EntityVersionRepo, creative *CreativeService, writing *WritingService) *VersionService {
	return &VersionService{repo: repo, creative: creative, writing: writing}
}

// OutlineTreeVersioner 是大纲树版本能力（由 OutlineService 实现）。
type OutlineTreeVersioner interface {
	TreePayload(ctx context.Context, outlineID string) (map[string]any, error)
	SnapshotTree(ctx context.Context, outlineID, note string) (*domain.EntityVersion, error)
	RestoreVersion(ctx context.Context, outlineID string, no int) (*OutlineDetail, error)
}

// SetOutlineTreeVersioner 注入大纲树版本能力（可选：未注入时大纲树版本接口返回明确错误）。
func (s *VersionService) SetOutlineTreeVersioner(v OutlineTreeVersioner) { s.outlineTree = v }

// ---------- 快照载荷 ----------

type characterSnapshot struct {
	Name          string                     `json:"name"`
	Description   string                     `json:"description"`
	Importance    int                        `json:"importance"`
	DNA           domain.CharacterDNA        `json:"dna"`
	FusionSources []domain.FusionSource      `json:"fusion_sources"`
	FusionDetail  []domain.FusionAttribution `json:"fusion_detail"`
	IsLocked      bool                       `json:"is_locked"`
	SourceType    domain.CreativeSourceType  `json:"source_type"`
}

type worldRuleSnapshot struct {
	ID          string                         `json:"id"`
	Category    string                         `json:"category"`
	Name        string                         `json:"name"`
	Description string                         `json:"description"`
	Importance  int                            `json:"importance"`
	Status      domain.CreativeWorldRuleStatus `json:"status"`
}

type worldSnapshot struct {
	World *worldSnapshotBody  `json:"world"`
	Rules []worldRuleSnapshot `json:"rules"`
}

type worldSnapshotBody struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	InheritMode string `json:"inherit_mode"`
}

type outlineChapterSnapshot struct {
	ID        string  `json:"id"`
	VolumeID  *string `json:"volume_id"`
	ChapterNo int     `json:"chapter_no"`
	Title     string  `json:"title"`
	Summary   string  `json:"summary"`
	Purpose   string  `json:"purpose"`
	Conflict  string  `json:"conflict"`
	Outcome   string  `json:"outcome"`
	Status    string  `json:"status"`
}

type outlineVolumeSnapshot struct {
	ID       string `json:"id"`
	Title    string `json:"title"`
	Summary  string `json:"summary"`
	Sequence int    `json:"sequence"`
}

type outlineSnapshot struct {
	Volumes  []outlineVolumeSnapshot  `json:"volumes"`
	Chapters []outlineChapterSnapshot `json:"chapters"`
}

// ---------- 快照 ----------

// SnapshotCharacter 存一份人物快照（内容与上一版相同则跳过）。
func (s *VersionService) SnapshotCharacter(ctx context.Context, characterID, note string) (*domain.EntityVersion, error) {
	payload, workID, err := s.buildCharacterPayloadWithWork(ctx, characterID)
	if err != nil {
		return nil, err
	}
	return s.save(ctx, domain.VersionCreativeCharacter, characterID, workID, payload, note)
}

// buildCharacterPayload 只组装快照内容，不写库（供快照与「版本比较」共用）。
func (s *VersionService) buildCharacterPayload(ctx context.Context, characterID string) (map[string]any, error) {
	payload, _, err := s.buildCharacterPayloadWithWork(ctx, characterID)
	return payload, err
}

func (s *VersionService) buildCharacterPayloadWithWork(ctx context.Context, characterID string) (map[string]any, string, error) {
	detail, err := s.creative.GetCharacter(ctx, characterID)
	if err != nil {
		return nil, "", err
	}
	c := detail.Character
	payload, err := toPayload(characterSnapshot{
		Name: c.Name, Description: c.Description, Importance: c.Importance, DNA: c.DNA,
		FusionSources: c.FusionSources, FusionDetail: c.FusionDetail, IsLocked: c.IsLocked,
		SourceType: c.SourceType,
	})
	if err != nil {
		return nil, "", err
	}
	return payload, c.CreativeWorkID, nil
}

// SnapshotWorld 存一份世界观快照（世界设定 + 全部规则）。
func (s *VersionService) SnapshotWorld(ctx context.Context, workID, note string) (*domain.EntityVersion, error) {
	payload, err := s.buildWorldPayload(ctx, workID)
	if err != nil {
		return nil, err
	}
	return s.save(ctx, domain.VersionCreativeWorld, workID, workID, payload, note)
}

// buildWorldPayload 只组装世界观快照内容，不写库。
func (s *VersionService) buildWorldPayload(ctx context.Context, workID string) (map[string]any, error) {
	detail, err := s.creative.GetWorldDetail(ctx, workID)
	if err != nil {
		return nil, err
	}
	snap := worldSnapshot{Rules: []worldRuleSnapshot{}}
	if detail.World != nil {
		snap.World = &worldSnapshotBody{
			Name: detail.World.Name, Description: detail.World.Description,
			InheritMode: string(detail.World.InheritanceMode),
		}
	}
	for _, r := range detail.Rules {
		snap.Rules = append(snap.Rules, worldRuleSnapshot{
			ID: r.ID, Category: r.Category, Name: r.Name, Description: r.Description,
			Importance: r.Importance, Status: r.Status,
		})
	}
	payload, err := toPayload(snap)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// SnapshotOutline 存一份大纲快照（卷 + 章节的大纲字段，不含正文）。
func (s *VersionService) SnapshotOutline(ctx context.Context, workID, note string) (*domain.EntityVersion, error) {
	payload, err := s.buildOutlinePayload(ctx, workID)
	if err != nil {
		return nil, err
	}
	return s.save(ctx, domain.VersionCreativeOutline, workID, workID, payload, note)
}

// buildOutlinePayload 只组装大纲快照内容，不写库。
func (s *VersionService) buildOutlinePayload(ctx context.Context, workID string) (map[string]any, error) {
	if _, err := s.creative.GetWork(ctx, workID); err != nil {
		return nil, err
	}
	volumes, err := s.writing.ListVolumes(ctx, workID)
	if err != nil {
		return nil, err
	}
	chapters, err := s.writing.ListChapters(ctx, workID, false)
	if err != nil {
		return nil, err
	}
	snap := outlineSnapshot{Volumes: []outlineVolumeSnapshot{}, Chapters: []outlineChapterSnapshot{}}
	for _, v := range volumes {
		snap.Volumes = append(snap.Volumes, outlineVolumeSnapshot{
			ID: v.ID, Title: v.Title, Summary: v.Summary, Sequence: v.Sequence,
		})
	}
	for _, c := range chapters {
		snap.Chapters = append(snap.Chapters, outlineChapterSnapshot{
			ID: c.ID, VolumeID: c.VolumeID, ChapterNo: c.ChapterNo, Title: c.Title, Summary: c.Summary,
			Purpose: c.Purpose, Conflict: c.Conflict, Outcome: c.Outcome, Status: string(c.Status),
		})
	}
	payload, err := toPayload(snap)
	if err != nil {
		return nil, err
	}
	return payload, nil
}

// save 落库；与最新一版内容一致时不产生新版本（返回 nil, nil）。
func (s *VersionService) save(
	ctx context.Context,
	t domain.EntityVersionType,
	entityID, workID string,
	payload map[string]any,
	note string,
) (*domain.EntityVersion, error) {
	latest, err := s.repo.Latest(ctx, t, entityID)
	if err != nil {
		return nil, err
	}
	if latest != nil && reflect.DeepEqual(latest.Payload, payload) {
		return nil, nil
	}
	no, err := s.repo.NextVersionNo(ctx, t, entityID)
	if err != nil {
		return nil, err
	}
	version := &domain.EntityVersion{
		EntityType: t, EntityID: entityID, CreativeWorkID: workID,
		VersionNo: no, Payload: payload, Note: strings.TrimSpace(note),
	}
	if err := s.repo.Create(ctx, version); err != nil {
		return nil, err
	}
	return version, nil
}

// ---------- 查询 ----------

// List 列出某实体的版本（新到旧）。
func (s *VersionService) List(ctx context.Context, t, entityID string) ([]domain.EntityVersion, error) {
	kind, err := parseVersionType(t)
	if err != nil {
		return nil, err
	}
	items, err := s.repo.List(ctx, kind, entityID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.EntityVersion{}
	}
	return items, nil
}

// Get 取某个版本（含快照内容）。
func (s *VersionService) Get(ctx context.Context, t, entityID string, no int) (*domain.EntityVersion, error) {
	kind, err := parseVersionType(t)
	if err != nil {
		return nil, err
	}
	return s.repo.GetByNo(ctx, kind, entityID, no)
}

// Restore 恢复到某个版本。
func (s *VersionService) Restore(ctx context.Context, t, entityID string, no int) error {
	kind, err := parseVersionType(t)
	if err != nil {
		return err
	}
	version, err := s.repo.GetByNo(ctx, kind, entityID, no)
	if err != nil {
		return err
	}
	// 大纲树的恢复由大纲服务负责（整树替换 + 恢复前后的自动备份），这里只做转发。
	if kind == domain.VersionCreativeOutlineTree {
		if s.outlineTree == nil {
			return fmt.Errorf("%w: %s", domain.ErrVersionTypeBad, kind)
		}
		_, err := s.outlineTree.RestoreVersion(ctx, entityID, no)
		return err
	}
	// 恢复前先把"现状"留一版，误点恢复也能退回去。
	//
	// 审查 P1-5：这里原本是 `_, _ =`（吞掉错误）。备份失败却继续恢复，
	// 等于"误恢复可退回"的能力静默消失，作者直到想退回时才发现没有那一版。备份失败必须中止恢复。
	if err := s.snapshotBeforeRestore(ctx, kind, entityID, no); err != nil {
		return fmt.Errorf("恢复前的自动备份失败，已中止恢复：%w", err)
	}

	switch kind {
	case domain.VersionCreativeCharacter:
		err = s.restoreCharacter(ctx, entityID, version.Payload)
	case domain.VersionCreativeWorld:
		err = s.restoreWorld(ctx, entityID, version.Payload)
	case domain.VersionCreativeOutline:
		err = s.restoreOutline(ctx, entityID, version.Payload)
	}
	if err != nil {
		return err
	}

	// 恢复后的状态再留一版
	switch kind {
	case domain.VersionCreativeCharacter:
		_, err = s.SnapshotCharacter(ctx, entityID, fmt.Sprintf("恢复自 v%d", no))
	case domain.VersionCreativeWorld:
		_, err = s.SnapshotWorld(ctx, entityID, fmt.Sprintf("恢复自 v%d", no))
	case domain.VersionCreativeOutline:
		_, err = s.SnapshotOutline(ctx, entityID, fmt.Sprintf("恢复自 v%d", no))
	}
	return err
}

// snapshotBeforeRestore 在恢复前把当前状态存一版（按实体类型分发）。
func (s *VersionService) snapshotBeforeRestore(ctx context.Context, kind domain.EntityVersionType, entityID string, no int) error {
	note := fmt.Sprintf("恢复 v%d 前的自动备份", no)
	var err error
	switch kind {
	case domain.VersionCreativeCharacter:
		_, err = s.SnapshotCharacter(ctx, entityID, note)
	case domain.VersionCreativeWorld:
		_, err = s.SnapshotWorld(ctx, entityID, note)
	case domain.VersionCreativeOutline:
		_, err = s.SnapshotOutline(ctx, entityID, note)
	}
	return err
}

func (s *VersionService) restoreCharacter(ctx context.Context, characterID string, payload map[string]any) error {
	var snap characterSnapshot
	if err := fromPayload(payload, &snap); err != nil {
		return err
	}
	current, err := s.creative.GetCharacter(ctx, characterID)
	if err != nil {
		return err
	}
	c := &current.Character
	c.Name = snap.Name
	c.Description = snap.Description
	c.Importance = snap.Importance
	c.DNA = snap.DNA
	c.FusionSources = snap.FusionSources
	c.FusionDetail = snap.FusionDetail
	c.IsLocked = snap.IsLocked
	c.Normalize()
	return s.creative.SaveCharacter(ctx, c)
}

func (s *VersionService) restoreWorld(ctx context.Context, workID string, payload map[string]any) error {
	var snap worldSnapshot
	if err := fromPayload(payload, &snap); err != nil {
		return err
	}
	if snap.World != nil {
		mode := domain.WorldInheritanceMode(snap.World.InheritMode)
		if !mode.Valid() {
			mode = domain.WorldInheritPartial
		}
		name, description := snap.World.Name, snap.World.Description
		if _, err := s.creative.UpdateWorld(ctx, workID, UpdateWorldInput{
			Name: &name, Description: &description, InheritanceMode: &mode,
		}); err != nil {
			return err
		}
	}
	for _, r := range snap.Rules {
		status := r.Status
		if !status.Valid() {
			status = domain.RuleInherited
		}
		if _, err := s.creative.UpdateWorldRule(ctx, r.ID, WorldRuleInput{
			Category: r.Category, Name: r.Name, Description: r.Description,
			Importance: r.Importance, Status: status,
		}); err != nil {
			// 规则可能已被删除：这种情况跳过，不让整次恢复失败
			if errors.Is(err, domain.ErrCreativeWorldRuleNotFound) {
				continue
			}
			return err
		}
	}
	return nil
}

func (s *VersionService) restoreOutline(ctx context.Context, workID string, payload map[string]any) error {
	var snap outlineSnapshot
	if err := fromPayload(payload, &snap); err != nil {
		return err
	}
	for _, c := range snap.Chapters {
		chapter, err := s.writing.GetChapter(ctx, c.ID)
		if err != nil {
			if errors.Is(err, domain.ErrCreativeChapterNotFound) {
				continue
			}
			return err
		}
		status := domain.ChapterStatus(c.Status)
		if !status.Valid() {
			status = domain.ChapterDraft
		}
		volumeID := c.VolumeID
		if _, _, err := s.writing.UpdateChapter(ctx, c.ID, UpdateChapterInput{
			Title: &c.Title, Summary: &c.Summary, VolumeID: volumeID,
			Purpose: &c.Purpose, Conflict: &c.Conflict, Outcome: &c.Outcome, Status: &status,
		}); err != nil {
			return err
		}
		_ = chapter
	}
	return nil
}

func parseVersionType(t string) (domain.EntityVersionType, error) {
	kind := domain.EntityVersionType(strings.TrimSpace(t))
	if !kind.Valid() {
		return "", fmt.Errorf("%w: %s", domain.ErrVersionTypeBad, t)
	}
	return kind, nil
}

func toPayload(v any) (map[string]any, error) {
	raw, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("生成快照失败: %w", err)
	}
	out := map[string]any{}
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("生成快照失败: %w", err)
	}
	return out, nil
}

func fromPayload(payload map[string]any, out any) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("解析快照失败: %w", err)
	}
	if err := json.Unmarshal(raw, out); err != nil {
		return fmt.Errorf("解析快照失败: %w", err)
	}
	return nil
}
