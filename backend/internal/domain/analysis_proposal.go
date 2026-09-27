package domain

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// 分析提案相关错误。
var (
	ErrProposalNotFound       = errors.New("分析提案不存在")
	ErrProposalAlreadyDecided = errors.New("该提案已经审核过")
	ErrProposalStageInvalid   = errors.New("分析阶段非法")
	ErrProposalEntityInvalid  = errors.New("提案实体类型非法")
	ErrProposalPayloadInvalid = errors.New("提案内容不合法")
)

// AnalysisStage 是分析阶段（规格书 §54 的分阶段任务）。
type AnalysisStage string

const (
	StageChapterSummary   AnalysisStage = "chapter_summary"
	StageCharacterExtract AnalysisStage = "character_extract"
	StageWorldExtract     AnalysisStage = "world_extract"
	StagePlotExtract      AnalysisStage = "plot_extract"
)

// Valid 判断阶段是否合法。
func (s AnalysisStage) Valid() bool {
	switch s {
	case StageChapterSummary, StageCharacterExtract, StageWorldExtract, StagePlotExtract:
		return true
	default:
		return false
	}
}

// TaskType 返回该阶段对应的任务类型名。
func (s AnalysisStage) TaskType() string { return "analysis_" + string(s) }

// ProposalEntity 是提案对应的实体类型。
type ProposalEntity string

const (
	EntityChapterSummary ProposalEntity = "chapter_summary"
	EntityCharacter      ProposalEntity = "character"
	EntityWorld          ProposalEntity = "world"
	EntityWorldRule      ProposalEntity = "world_rule"
	EntityLocation       ProposalEntity = "location"
	EntityFaction        ProposalEntity = "faction"
	EntityEvent          ProposalEntity = "event"
	EntityPlotArc        ProposalEntity = "plot_arc"
)

// Valid 判断实体类型是否合法。
func (e ProposalEntity) Valid() bool {
	switch e {
	case EntityChapterSummary, EntityCharacter, EntityWorld, EntityWorldRule, EntityLocation,
		EntityFaction, EntityEvent, EntityPlotArc:
		return true
	default:
		return false
	}
}

// ProposalStatus 是提案状态。
type ProposalStatus string

const (
	ProposalPending  ProposalStatus = "PENDING"
	ProposalApproved ProposalStatus = "APPROVED"
	ProposalRejected ProposalStatus = "REJECTED"
)

// Valid 判断状态是否合法。
func (s ProposalStatus) Valid() bool {
	return s == ProposalPending || s == ProposalApproved || s == ProposalRejected
}

// AnalysisProposal 是一条 AI 提案。
type AnalysisProposal struct {
	ID         string
	WorkID     string
	TaskID     *string
	Stage      AnalysisStage
	EntityType ProposalEntity
	Title      string
	Payload    map[string]any
	Evidence   string
	Confidence int
	Status     ProposalStatus
	ReviewNote string
	ReviewedAt *time.Time
	AppliedID  *string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

// Normalize 清洗输入并补默认值。
func (p *AnalysisProposal) Normalize() {
	p.Title = strings.TrimSpace(p.Title)
	p.Evidence = strings.TrimSpace(p.Evidence)
	if p.Payload == nil {
		p.Payload = map[string]any{}
	}
	if p.Status == "" {
		p.Status = ProposalPending
	}
}

// Validate 校验提案；不同实体类型要求的最小字段不同 —— 这里挡住"模型返回了半截 JSON 就入库"。
func (p *AnalysisProposal) Validate() error {
	if !p.Stage.Valid() {
		return fmt.Errorf("%w: %s", ErrProposalStageInvalid, p.Stage)
	}
	if !p.EntityType.Valid() {
		return fmt.Errorf("%w: %s", ErrProposalEntityInvalid, p.EntityType)
	}
	return ValidateProposalPayload(p.EntityType, p.Payload)
}

// ValidateProposalPayload 校验提案载荷的最小必要字段。
func ValidateProposalPayload(entity ProposalEntity, payload map[string]any) error {
	requireString := func(key string) error {
		v, ok := payload[key]
		if !ok {
			return fmt.Errorf("%w: 缺少字段 %s", ErrProposalPayloadInvalid, key)
		}
		s, ok := v.(string)
		if !ok || strings.TrimSpace(s) == "" {
			return fmt.Errorf("%w: 字段 %s 不能为空", ErrProposalPayloadInvalid, key)
		}
		return nil
	}

	switch entity {
	case EntityChapterSummary:
		return requireString("summary")
	case EntityCharacter:
		return requireString("name")
	case EntityWorld:
		// 世界名可以为空（只更新描述也是合法的），这里不强制
		return nil
	case EntityWorldRule:
		return requireString("name")
	case EntityLocation:
		return requireString("name")
	case EntityFaction:
		return requireString("name")
	case EntityEvent:
		return requireString("title")
	case EntityPlotArc:
		if err := requireString("title"); err != nil {
			return err
		}
		raw, ok := payload["type"]
		if !ok {
			return nil // 类型缺省由服务层补 main
		}
		s, _ := raw.(string)
		if s != "" && !PlotArcType(s).Valid() {
			return fmt.Errorf("%w: 剧情弧类型非法 %s", ErrProposalPayloadInvalid, s)
		}
		return nil
	default:
		return fmt.Errorf("%w: 未知实体类型 %s", ErrProposalPayloadInvalid, entity)
	}
}
