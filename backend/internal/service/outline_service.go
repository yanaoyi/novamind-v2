package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
)

// OutlineRepository 是大纲服务需要的仓储能力（规格书 §27）。
type OutlineRepository interface {
	CreateOutline(ctx context.Context, o *domain.Outline) error
	ListOutlines(ctx context.Context, workID string) ([]domain.Outline, error)
	GetOutline(ctx context.Context, id string) (*domain.Outline, error)
	UpdateOutline(ctx context.Context, o *domain.Outline) error
	DeleteOutline(ctx context.Context, id string) error

	ListNodes(ctx context.Context, outlineID string) ([]domain.OutlineNode, error)
	GetNode(ctx context.Context, id string) (*domain.OutlineNode, error)
	CreateNode(ctx context.Context, n *domain.OutlineNode) error
	// CreateNodeLocked 取号 + 写入在同一事务内串行化（审查 P2：并发建节点序号会撞）
	CreateNodeLocked(ctx context.Context, n *domain.OutlineNode) error
	UpdateNode(ctx context.Context, n *domain.OutlineNode) error
	DeleteNodeSubtree(ctx context.Context, id string) (int, error)
	NextNodeSequence(ctx context.Context, outlineID string, parentID *string) (int, error)
	ReplaceTree(ctx context.Context, outlineID string, nodes []domain.OutlineNode, parents []int) ([]domain.OutlineNode, error)
	// RestoreTreeWithMeta 整树替换 + 元信息回填，同一事务（审查 P1-7）
	RestoreTreeWithMeta(ctx context.Context, outline *domain.Outline, nodes []domain.OutlineNode, parents []int) ([]domain.OutlineNode, error)
}

// OutlineMaterializer 是「大纲落成卷与章节」需要的最小写作能力（由 WritingRepo 提供）。
type OutlineMaterializer interface {
	ListVolumes(ctx context.Context, workID string) ([]domain.CreativeVolume, error)
	ListChapters(ctx context.Context, workID string, withContent bool) ([]domain.CreativeChapter, error)
	// MaterializeOutline 在**一次事务**里写完新建的卷与章节（半成品不可接受，见 P1-2）
	MaterializeOutline(ctx context.Context, workID string, plan repository.MaterializePlan) (*repository.MaterializeOutcome, error)
}

// OutlineService 是大纲服务（规格书 §27；§68 的「生成二创大纲 → 生成章节」）。
type OutlineService struct {
	repo     OutlineRepository
	creative CreativeWorkReader
	writing  OutlineMaterializer
	versions *repository.EntityVersionRepo
	// indexer 是"大纲节点变了就重建索引"的触发点（Phase 9 §9.1.4）。
	indexer IndexTrigger
}

// NewOutlineService 构建服务。
func NewOutlineService(
	repo OutlineRepository,
	creative CreativeWorkReader,
	writing OutlineMaterializer,
	versions *repository.EntityVersionRepo,
) *OutlineService {
	return &OutlineService{repo: repo, creative: creative, writing: writing, versions: versions}
}

// SetIndexTrigger 注入索引入队能力（Phase 9 §9.1.4）。
func (s *OutlineService) SetIndexTrigger(t IndexTrigger) { s.indexer = t }

// triggerIndex 入队重建某个大纲节点的索引；refID 为空表示按作品全量重建。
// 失败只记日志：索引是派生数据，不该阻塞作者改大纲。
func (s *OutlineService) triggerIndex(ctx context.Context, workID, refID string) {
	if s.indexer == nil || strings.TrimSpace(workID) == "" {
		return
	}
	refKind := ""
	if refID != "" {
		refKind = domain.ChunkRefOutlineNode
	}
	if err := s.indexer.EnqueueIndex(ctx, domain.WorkKindCreative, workID, refKind, refID); err != nil {
		fmt.Printf("[warn] 大纲索引入队失败（不影响保存）: work=%s node=%s %v\n", workID, refID, err)
	}
}

// triggerIndexByOutline 用大纲 ID 反查作品后入队（大纲→作品要查一次，集中在这里）。
func (s *OutlineService) triggerIndexByOutline(ctx context.Context, outlineID, nodeID string) {
	if s.indexer == nil || strings.TrimSpace(outlineID) == "" {
		return
	}
	outline, err := s.repo.GetOutline(ctx, outlineID)
	if err != nil {
		return
	}
	s.triggerIndex(ctx, outline.CreativeWorkID, nodeID)
}

// OutlineDetail 是一份大纲及其完整节点树。
type OutlineDetail struct {
	Outline domain.Outline
	Nodes   []*domain.OutlineNodeTree
}

// CreateOutlineInput 是新建大纲的入参。
//
// Nodes 为空时创建一份空大纲（作者从零搭结构）；非空时一次性写入整棵树，
// 这也是「采纳 AI 候选」的落库路径——AI 产出的候选由作者确认后走这里进来（§52 红线）。
type CreateOutlineInput struct {
	Title   string
	Summary string
	Version int
	Source  domain.OutlineSource
	Nodes   []domain.OutlineNodeInput
}

// CreateOutline 新建大纲（可同时写入整棵树）。
func (s *OutlineService) CreateOutline(ctx context.Context, workID string, in CreateOutlineInput) (*OutlineDetail, error) {
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	outline := &domain.Outline{
		CreativeWorkID: workID, Title: in.Title, Summary: in.Summary,
		Version: in.Version, Source: in.Source,
	}
	outline.Normalize()
	if err := outline.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateOutline(ctx, outline); err != nil {
		return nil, err
	}
	if len(in.Nodes) > 0 {
		if err := s.replaceTree(ctx, outline, in.Nodes); err != nil {
			return nil, err
		}
	}
	return s.GetOutline(ctx, outline.ID)
}

// ListOutlines 列出某作品的全部大纲。
func (s *OutlineService) ListOutlines(ctx context.Context, workID string) ([]domain.Outline, error) {
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return nil, err
	}
	items, err := s.repo.ListOutlines(ctx, workID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.Outline{}
	}
	return items, nil
}

// GetOutline 取大纲详情（含树）。
func (s *OutlineService) GetOutline(ctx context.Context, outlineID string) (*OutlineDetail, error) {
	outline, err := s.repo.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	nodes, err := s.repo.ListNodes(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	outline.NodeCount = len(nodes)
	return &OutlineDetail{Outline: *outline, Nodes: domain.BuildOutlineTree(nodes)}, nil
}

// UpdateOutlineInput 是大纲元信息修改入参（只允许改标题 / 概要 / 版本号）。
type UpdateOutlineInput struct {
	Title   *string
	Summary *string
	Version *int
}

// UpdateOutline 修改大纲元信息。
func (s *OutlineService) UpdateOutline(ctx context.Context, outlineID string, in UpdateOutlineInput) (*OutlineDetail, error) {
	outline, err := s.repo.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	if in.Title != nil {
		outline.Title = *in.Title
	}
	if in.Summary != nil {
		outline.Summary = *in.Summary
	}
	if in.Version != nil {
		outline.Version = *in.Version
	}
	outline.Normalize()
	if err := outline.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateOutline(ctx, outline); err != nil {
		return nil, err
	}
	return s.GetOutline(ctx, outlineID)
}

// DeleteOutline 删除大纲（连同节点）。
func (s *OutlineService) DeleteOutline(ctx context.Context, outlineID string) error {
	return s.repo.DeleteOutline(ctx, outlineID)
}

// ReplaceTree 用作者提交的整棵树替换大纲内容（AI 候选改完再提交也走这里）。
func (s *OutlineService) ReplaceTree(ctx context.Context, outlineID string, inputs []domain.OutlineNodeInput) (*OutlineDetail, error) {
	outline, err := s.repo.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	if err := s.replaceTree(ctx, outline, inputs); err != nil {
		return nil, err
	}
	// 整棵树换掉了 → 节点 ref 全变，直接按作品全量重建
	s.triggerIndex(ctx, outline.CreativeWorkID, "")
	return s.GetOutline(ctx, outlineID)
}

func (s *OutlineService) replaceTree(ctx context.Context, outline *domain.Outline, inputs []domain.OutlineNodeInput) error {
	nodes, parents, err := domain.FlattenOutlineInput(inputs)
	if err != nil {
		return err
	}
	if _, err := s.repo.ReplaceTree(ctx, outline.ID, nodes, parents); err != nil {
		return err
	}
	return nil
}

// CreateNodeInput 是新增单个大纲节点的入参。
type CreateNodeInput struct {
	ParentID   *string
	Title      string
	Summary    string
	Purpose    string
	Characters []string
	Location   string
	Conflict   string
	Outcome    string
}

// GetNode 取单个大纲节点。
func (s *OutlineService) GetNode(ctx context.Context, nodeID string) (*domain.OutlineNode, error) {
	return s.repo.GetNode(ctx, nodeID)
}

// CreateNode 在指定父节点下新增一个节点；层级由父节点推导（卷 → 节 → 章）。
func (s *OutlineService) CreateNode(ctx context.Context, outlineID string, in CreateNodeInput) (*domain.OutlineNode, error) {
	if _, err := s.repo.GetOutline(ctx, outlineID); err != nil {
		return nil, err
	}
	level := domain.OutlineLevelVolume
	if in.ParentID != nil {
		parent, err := s.repo.GetNode(ctx, *in.ParentID)
		if err != nil {
			return nil, err
		}
		if parent.OutlineID != outlineID {
			return nil, domain.ErrOutlineParentNotInTree
		}
		if parent.Level >= domain.OutlineLevelChapter {
			return nil, fmt.Errorf("%w：章节点下面不能再加子节点", domain.ErrOutlineLevelJumpInvalid)
		}
		level = parent.Level + 1
	}
	node := &domain.OutlineNode{
		// Sequence 交给仓储在事务里取（并发安全），这里不预取
		OutlineID: outlineID, ParentID: in.ParentID, Level: level,
		Title: in.Title, Summary: in.Summary, Purpose: in.Purpose, Characters: in.Characters,
		Location: in.Location, Conflict: in.Conflict, Outcome: in.Outcome,
	}
	node.Normalize()
	if err := node.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.CreateNodeLocked(ctx, node); err != nil {
		return nil, err
	}
	s.triggerIndexByOutline(ctx, outlineID, node.ID)
	return node, nil
}

// UpdateNodeInput 是节点修改入参（nil 字段表示不改）。
type UpdateNodeInput struct {
	Title      *string
	Summary    *string
	Purpose    *string
	Characters *[]string
	Location   *string
	Conflict   *string
	Outcome    *string
	Sequence   *int
}

// UpdateNode 修改节点。
func (s *OutlineService) UpdateNode(ctx context.Context, nodeID string, in UpdateNodeInput) (*domain.OutlineNode, error) {
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return nil, err
	}
	if in.Title != nil {
		node.Title = *in.Title
	}
	if in.Summary != nil {
		node.Summary = *in.Summary
	}
	if in.Purpose != nil {
		node.Purpose = *in.Purpose
	}
	if in.Characters != nil {
		node.Characters = *in.Characters
	}
	if in.Location != nil {
		node.Location = *in.Location
	}
	if in.Conflict != nil {
		node.Conflict = *in.Conflict
	}
	if in.Outcome != nil {
		node.Outcome = *in.Outcome
	}
	if in.Sequence != nil && *in.Sequence > 0 {
		node.Sequence = *in.Sequence
	}
	node.Normalize()
	if err := node.Validate(); err != nil {
		return nil, err
	}
	if err := s.repo.UpdateNode(ctx, node); err != nil {
		return nil, err
	}
	s.triggerIndexByOutline(ctx, node.OutlineID, node.ID)
	return node, nil
}

// DeleteNode 删除节点及其子树，返回删除数量。
func (s *OutlineService) DeleteNode(ctx context.Context, nodeID string) (int, error) {
	// 先取一次拿大纲归属：删掉之后就查不到它属于哪份大纲了
	node, err := s.repo.GetNode(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	deleted, err := s.repo.DeleteNodeSubtree(ctx, nodeID)
	if err != nil {
		return 0, err
	}
	// 子树一起被删了 → 按作品全量重建最稳妥（逐节点清块容易漏）
	s.triggerIndexByOutline(ctx, node.OutlineID, "")
	return deleted, nil
}

// MaterializeResult 是「大纲落成章节」的结果。
type MaterializeResult struct {
	VolumesCreated  int      `json:"volumes_created"`
	VolumesReused   int      `json:"volumes_reused"`
	ChaptersCreated int      `json:"chapters_created"`
	ChaptersSkipped int      `json:"chapters_skipped"`
	ChapterIDs      []string `json:"chapter_ids"`
}

// Materialize 把大纲里的「章」节点落成写作系统的卷与章节（规格书 §68 的下一步）。
//
// 语义（刻意保守）：
//   - 卷按标题复用：已有同名卷就挂上去，不重复建；
//   - 章节**按来源节点防重**：同一个大纲节点只落成一章，重复点「落成章节」只补新增的，
//     已经落成过的节点跳过（章号从现有最大章号继续排），不覆盖、不删除作者已经写好的内容；
//   - 「节」在写作系统里没有对应表（§28 只有卷→章），因此章节点若挂在节下面，
//     会把节标题作为摘要前缀保留下来，避免结构信息凭空消失。
func (s *OutlineService) Materialize(ctx context.Context, outlineID string) (*MaterializeResult, error) {
	detail, err := s.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	workID := detail.Outline.CreativeWorkID

	volumes, err := s.writing.ListVolumes(ctx, workID)
	if err != nil {
		return nil, err
	}
	chapters, err := s.writing.ListChapters(ctx, workID, false)
	if err != nil {
		return nil, err
	}
	volumeByTitle := make(map[string]string, len(volumes))
	for _, v := range volumes {
		volumeByTitle[v.Title] = v.ID
	}
	nextChapterNo := 1
	for _, c := range chapters {
		if c.ChapterNo >= nextChapterNo {
			nextChapterNo = c.ChapterNo + 1
		}
	}

	result := &MaterializeResult{ChapterIDs: []string{}}
	volumeOrder := len(volumes) + 1
	// 已经落成过的大纲节点集合（防重）
	alreadyDone := make(map[string]bool, len(chapters))
	maxChapterNo := 0
	for _, c := range chapters {
		if c.OutlineNodeID != nil {
			alreadyDone[*c.OutlineNodeID] = true
		}
		if c.ChapterNo > maxChapterNo {
			maxChapterNo = c.ChapterNo
		}
	}
	nextChapterNo = maxChapterNo + 1

	// 先把计划算出来（哪些卷要新建、哪些章节要写），再交给仓储一次事务写完
	plan := repository.MaterializePlan{}
	for _, v := range volumes {
		plan.ExistingVolumeIDs = append(plan.ExistingVolumeIDs, v.ID)
	}
	newVolumeIndex := make(map[string]int) // 卷标题 → plan.NewVolumes 下标

	var walk func(nodes []*domain.OutlineNodeTree, volumeIndex int, sectionTitle string)
	walk = func(nodes []*domain.OutlineNodeTree, volumeIndex int, sectionTitle string) {
		for _, n := range nodes {
			switch n.Level {
			case domain.OutlineLevelVolume:
				childVolumeIndex := volumeIndex
				id, ok := volumeByTitle[n.Title]
				if ok {
					result.VolumesReused++
					idx := -1
					for i, existing := range volumes {
						if existing.ID == id {
							idx = i
							break
						}
					}
					childVolumeIndex = idx
				} else {
					idx, planned := newVolumeIndex[n.Title]
					if !planned {
						idx = len(plan.NewVolumes)
						newVolumeIndex[n.Title] = idx
						plan.NewVolumes = append(plan.NewVolumes, repository.MaterializeVolume{
							Title: n.Title, Summary: n.Summary, Sequence: volumeOrder,
						})
						volumeByTitle[n.Title] = "" // 占位：本批次内同名卷只建一次
						volumeOrder++
						result.VolumesCreated++
					}
					// 新卷在 plan 里的下标要换算成"现有卷数量 + 新卷序号"
					childVolumeIndex = len(plan.ExistingVolumeIDs) + idx
				}
				walk(n.Children, childVolumeIndex, "")
			case domain.OutlineLevelSection:
				walk(n.Children, volumeIndex, n.Title)
			case domain.OutlineLevelChapter:
				if alreadyDone[n.ID] {
					result.ChaptersSkipped++
					continue
				}
				summary := n.Summary
				if sectionTitle != "" {
					prefix := fmt.Sprintf("【%s】", sectionTitle)
					if summary == "" {
						summary = prefix
					} else {
						summary = prefix + summary
					}
				}
				plan.Chapters = append(plan.Chapters, repository.MaterializeChapter{
					VolumeIndex: volumeIndex, ChapterNo: nextChapterNo, Title: n.Title, Summary: summary,
					Purpose: n.Purpose, Conflict: n.Conflict, Outcome: n.Outcome, OutlineNodeID: n.ID,
				})
				nextChapterNo++
			}
		}
	}
	walk(detail.Nodes, -1, "")

	if len(plan.Chapters) == 0 {
		return result, nil // 全部已落成，无事可做（不产生任何写入）
	}
	outcome, err := s.writing.MaterializeOutline(ctx, workID, plan)
	if err != nil {
		return nil, err
	}
	result.ChapterIDs = outcome.ChapterIDs
	result.ChaptersCreated = len(outcome.ChapterIDs)
	// 落成章节是直接写仓储（不走 WritingService），所以章节索引与记忆抽取都不会自动触发；
	// 这里至少把索引补上（章节是空的，真正有内容时作者一保存就会走正常链路）。
	s.triggerIndex(ctx, workID, "")
	return result, nil
}

// ---------- 版本历史（规格书 §59 的 Outline） ----------

type outlineTreeSnapshotNode struct {
	ID         string   `json:"id"`
	ParentID   *string  `json:"parent_id"`
	Level      int      `json:"level"`
	Sequence   int      `json:"sequence"`
	Title      string   `json:"title"`
	Summary    string   `json:"summary"`
	Purpose    string   `json:"purpose"`
	Characters []string `json:"characters"`
	Location   string   `json:"location"`
	Conflict   string   `json:"conflict"`
	Outcome    string   `json:"outcome"`
}

type outlineTreeSnapshot struct {
	Title   string                    `json:"title"`
	Summary string                    `json:"summary"`
	Version int                       `json:"version"`
	Source  string                    `json:"source"`
	Nodes   []outlineTreeSnapshotNode `json:"nodes"`
}

// TreePayload 组装大纲树的当前状态快照（供版本比较与快照共用）。
func (s *OutlineService) TreePayload(ctx context.Context, outlineID string) (map[string]any, error) {
	detail, err := s.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	flat, err := s.repo.ListNodes(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	snap := outlineTreeSnapshot{
		Title: detail.Outline.Title, Summary: detail.Outline.Summary,
		Version: detail.Outline.Version, Source: string(detail.Outline.Source),
		Nodes: make([]outlineTreeSnapshotNode, 0, len(flat)),
	}
	for _, n := range flat {
		snap.Nodes = append(snap.Nodes, outlineTreeSnapshotNode{
			ID: n.ID, ParentID: n.ParentID, Level: int(n.Level), Sequence: n.Sequence,
			Title: n.Title, Summary: n.Summary, Purpose: n.Purpose, Characters: n.Characters,
			Location: n.Location, Conflict: n.Conflict, Outcome: n.Outcome,
		})
	}
	return toPayload(snap)
}

// SnapshotTree 存一版大纲快照（内容与上一版相同则跳过）。
func (s *OutlineService) SnapshotTree(ctx context.Context, outlineID, note string) (*domain.EntityVersion, error) {
	if s.versions == nil {
		return nil, errors.New("版本仓储未配置")
	}
	payload, err := s.TreePayload(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	outline, err := s.repo.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	latest, err := s.versions.Latest(ctx, domain.VersionCreativeOutlineTree, outlineID)
	if err != nil {
		return nil, err
	}
	if latest != nil && shallowEqualJSON(latest.Payload, payload) {
		return nil, nil
	}
	no, err := s.versions.NextVersionNo(ctx, domain.VersionCreativeOutlineTree, outlineID)
	if err != nil {
		return nil, err
	}
	version := &domain.EntityVersion{
		EntityType: domain.VersionCreativeOutlineTree, EntityID: outlineID,
		CreativeWorkID: outline.CreativeWorkID, VersionNo: no, Payload: payload,
		Note: strings.TrimSpace(note),
	}
	if err := s.versions.Create(ctx, version); err != nil {
		return nil, err
	}
	return version, nil
}

// ListVersions 列出大纲树的历史版本（新到旧）。
func (s *OutlineService) ListVersions(ctx context.Context, outlineID string) ([]domain.EntityVersion, error) {
	if s.versions == nil {
		return nil, errors.New("版本仓储未配置")
	}
	if _, err := s.repo.GetOutline(ctx, outlineID); err != nil {
		return nil, err
	}
	items, err := s.versions.List(ctx, domain.VersionCreativeOutlineTree, outlineID)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []domain.EntityVersion{}
	}
	return items, nil
}

// GetVersion 取某一版。
func (s *OutlineService) GetVersion(ctx context.Context, outlineID string, no int) (*domain.EntityVersion, error) {
	if s.versions == nil {
		return nil, errors.New("版本仓储未配置")
	}
	if no <= 0 {
		return nil, domain.ErrVersionNoInvalid
	}
	return s.versions.GetByNo(ctx, domain.VersionCreativeOutlineTree, outlineID, no)
}

// RestoreVersion 恢复大纲树到某一版。
//
// 语义与章节版本一致：恢复前先把现状留一版；恢复是**整棵树的替换**（大纲树本身就是快照内容）。
// 已经从这份大纲落成的章节不会被回滚——章节是下游产物，作者可能已经写了正文。
func (s *OutlineService) RestoreVersion(ctx context.Context, outlineID string, no int) (*OutlineDetail, error) {
	if s.versions == nil {
		return nil, errors.New("版本仓储未配置")
	}
	version, err := s.versions.GetByNo(ctx, domain.VersionCreativeOutlineTree, outlineID, no)
	if err != nil {
		return nil, err
	}
	if _, err := s.SnapshotTree(ctx, outlineID, fmt.Sprintf("恢复 v%d 前的自动备份", no)); err != nil {
		return nil, err
	}

	var snap outlineTreeSnapshot
	if err := fromPayload(version.Payload, &snap); err != nil {
		return nil, err
	}
	if len(snap.Nodes) == 0 {
		return nil, fmt.Errorf("%w：这一版是空大纲", domain.ErrOutlineTreeEmpty)
	}
	index := make(map[string]int, len(snap.Nodes))
	for i, n := range snap.Nodes {
		index[n.ID] = i
	}
	nodes := make([]domain.OutlineNode, 0, len(snap.Nodes))
	parents := make([]int, 0, len(snap.Nodes))
	for _, n := range snap.Nodes {
		if !domain.OutlineLevel(n.Level).Valid() {
			return nil, fmt.Errorf("%w: %d", domain.ErrOutlineLevelInvalid, n.Level)
		}
		parent := -1
		if n.ParentID != nil {
			idx, ok := index[*n.ParentID]
			if !ok {
				return nil, fmt.Errorf("快照里的父节点 %s 缺失，无法恢复", *n.ParentID)
			}
			parent = idx
		}
		nodes = append(nodes, domain.OutlineNode{
			Level: domain.OutlineLevel(n.Level), Sequence: n.Sequence, Title: n.Title,
			Summary: n.Summary, Purpose: n.Purpose, Characters: n.Characters,
			Location: n.Location, Conflict: n.Conflict, Outcome: n.Outcome,
		})
		parents = append(parents, parent)
	}
	outline, err := s.repo.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	outline.Title, outline.Summary, outline.Version = snap.Title, snap.Summary, snap.Version
	// 来源按快照回填（审查 P2）：DB 列是 NOT NULL DEFAULT 'MANUAL'，永远不会是空串，
	// 原来写 `if outline.Source == ""` 等于永不执行，快照里的 AI/MANUAL 标记被丢掉。
	if snap.Source != "" {
		outline.Source = domain.OutlineSource(snap.Source)
	}
	outline.Normalize()
	if err := outline.Validate(); err != nil {
		return nil, err
	}
	// 整树替换 + 元信息回填在同一事务里（审查 P1-7）
	if _, err := s.repo.RestoreTreeWithMeta(ctx, outline, nodes, parents); err != nil {
		return nil, err
	}

	out, err := s.GetOutline(ctx, outlineID)
	if err != nil {
		return nil, err
	}
	// 恢复后的状态也留一版；快照失败要如实报错（审查 P1-5），不能让作者以为历史完整
	if _, err := s.SnapshotTree(ctx, outlineID, fmt.Sprintf("恢复自 v%d", no)); err != nil {
		return nil, fmt.Errorf("恢复已完成，但记录恢复版本失败：%w", err)
	}
	return out, nil
}

// shallowEqualJSON 比较两份快照是否等价（用于跳过重复版本）。
func shallowEqualJSON(a, b map[string]any) bool {
	ra, err1 := json.Marshal(a)
	rb, err2 := json.Marshal(b)
	if err1 != nil || err2 != nil {
		return false
	}
	return string(ra) == string(rb)
}
