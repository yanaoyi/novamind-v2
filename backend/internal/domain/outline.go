package domain

import (
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"
)

// 大纲领域错误（规格书 §27）。
var (
	ErrOutlineNotFound         = errors.New("大纲不存在")
	ErrOutlineTitleEmpty       = errors.New("大纲标题不能为空")
	ErrOutlineVersionInvalid   = errors.New("大纲版本号必须为正整数")
	ErrOutlineSourceInvalid    = errors.New("大纲来源非法")
	ErrOutlineNodeNotFound     = errors.New("大纲节点不存在")
	ErrOutlineNodeTitleEmpty   = errors.New("大纲节点标题不能为空")
	ErrOutlineLevelInvalid     = errors.New("大纲层级只能取 1（卷）/ 2（节）/ 3（章）")
	ErrOutlineParentNotFound   = errors.New("父节点不存在")
	ErrOutlineParentNotInTree  = errors.New("父节点不属于这份大纲")
	ErrOutlineLevelJumpInvalid = errors.New("子节点层级必须正好比父节点深一层")
	ErrOutlineNodeHasChildren  = errors.New("该节点还有子节点，请先删除子节点")
	ErrOutlineTreeTooDeep      = errors.New("大纲层级最多三层（卷 → 节 → 章）")
	ErrOutlineTreeEmpty        = errors.New("大纲至少要有一个节点")
)

// OutlineLevel 是大纲节点层级（规格书 §27：卷 → 节 → 章）。
type OutlineLevel int

const (
	OutlineLevelVolume  OutlineLevel = 1 // 卷
	OutlineLevelSection OutlineLevel = 2 // 节
	OutlineLevelChapter OutlineLevel = 3 // 章
)

// Valid 判断层级是否合法。
func (l OutlineLevel) Valid() bool {
	return l == OutlineLevelVolume || l == OutlineLevelSection || l == OutlineLevelChapter
}

// Name 返回中文名（用于错误提示与前端展示）。
func (l OutlineLevel) Name() string {
	switch l {
	case OutlineLevelVolume:
		return "卷"
	case OutlineLevelSection:
		return "节"
	case OutlineLevelChapter:
		return "章"
	default:
		return "未知"
	}
}

// OutlineSource 说明这份大纲从哪来（作者手写 / 采纳 AI 候选）。
type OutlineSource string

const (
	OutlineSourceManual OutlineSource = "MANUAL"
	OutlineSourceAI     OutlineSource = "AI"
)

// Valid 判断来源是否合法。
func (s OutlineSource) Valid() bool {
	return s == OutlineSourceManual || s == OutlineSourceAI
}

// Outline 是一份二创大纲（规格书 §27）。
type Outline struct {
	ID             string
	CreativeWorkID string
	Title          string
	Summary        string
	Version        int
	Source         OutlineSource
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      *time.Time
	// NodeCount 只在列表视图填充，便于作者一眼看出哪份大纲有内容。
	NodeCount int
}

// Normalize 清洗输入并补默认值。
func (o *Outline) Normalize() {
	o.Title = strings.TrimSpace(o.Title)
	o.Summary = strings.TrimSpace(o.Summary)
	if o.Version == 0 {
		o.Version = 1
	}
	if o.Source == "" {
		o.Source = OutlineSourceManual
	}
}

// Validate 校验大纲。
func (o *Outline) Validate() error {
	if o.Title == "" {
		return ErrOutlineTitleEmpty
	}
	if o.Version <= 0 {
		return ErrOutlineVersionInvalid
	}
	if !o.Source.Valid() {
		return fmt.Errorf("%w: %s", ErrOutlineSourceInvalid, o.Source)
	}
	return nil
}

// OutlineNode 是大纲树上的一个节点（规格书 §27）。
type OutlineNode struct {
	ID         string
	OutlineID  string
	ParentID   *string
	Level      OutlineLevel
	Sequence   int
	Title      string
	Summary    string
	Purpose    string
	Characters []string
	Location   string
	Conflict   string
	Outcome    string
	CreatedAt  time.Time
	UpdatedAt  time.Time
	DeletedAt  *time.Time
}

// Normalize 清洗输入并补默认值。
func (n *OutlineNode) Normalize() {
	n.Title = strings.TrimSpace(n.Title)
	n.Summary = strings.TrimSpace(n.Summary)
	n.Purpose = strings.TrimSpace(n.Purpose)
	n.Location = strings.TrimSpace(n.Location)
	n.Conflict = strings.TrimSpace(n.Conflict)
	n.Outcome = strings.TrimSpace(n.Outcome)
	if n.Sequence == 0 {
		n.Sequence = 1
	}
	if n.Characters == nil {
		n.Characters = []string{}
	}
	cleaned := make([]string, 0, len(n.Characters))
	for _, c := range n.Characters {
		if t := strings.TrimSpace(c); t != "" {
			cleaned = append(cleaned, t)
		}
	}
	n.Characters = cleaned
}

// Validate 校验节点自身（父子关系由服务层校验）。
func (n *OutlineNode) Validate() error {
	if n.Title == "" {
		return ErrOutlineNodeTitleEmpty
	}
	if !n.Level.Valid() {
		return fmt.Errorf("%w: %d", ErrOutlineLevelInvalid, int(n.Level))
	}
	if n.Level == OutlineLevelVolume && n.ParentID != nil {
		return errors.New("卷节点不能有父节点")
	}
	if n.Level != OutlineLevelVolume && n.ParentID == nil {
		return fmt.Errorf("「%s」节点必须有父节点", n.Level.Name())
	}
	return nil
}

// OutlineNodeTree 是带子节点的大纲节点（读接口的返回形态）。
type OutlineNodeTree struct {
	OutlineNode
	Children []*OutlineNodeTree `json:"children"`
}

// BuildOutlineTree 把扁平节点组装成树：按 level 与 parent_id 归位，同级按 sequence 排序。
//
// 容错策略：父节点不在集合里的节点（理论上不该出现）提升为根节点，而不是整棵树读不出来——
// 作者宁可看到"结构有点怪"，也不能因为一条脏数据打不开大纲。
func BuildOutlineTree(nodes []OutlineNode) []*OutlineNodeTree {
	trees := make(map[string]*OutlineNodeTree, len(nodes))
	order := make([]string, 0, len(nodes))
	for _, n := range nodes {
		copied := n
		trees[n.ID] = &OutlineNodeTree{OutlineNode: copied, Children: []*OutlineNodeTree{}}
		order = append(order, n.ID)
	}

	roots := make([]*OutlineNodeTree, 0, len(nodes))
	for _, id := range order {
		node := trees[id]
		if node.ParentID != nil {
			if parent, ok := trees[*node.ParentID]; ok {
				parent.Children = append(parent.Children, node)
				continue
			}
		}
		roots = append(roots, node)
	}

	var sortLevel func(items []*OutlineNodeTree)
	sortLevel = func(items []*OutlineNodeTree) {
		sort.SliceStable(items, func(i, j int) bool {
			if items[i].Sequence != items[j].Sequence {
				return items[i].Sequence < items[j].Sequence
			}
			return items[i].CreatedAt.Before(items[j].CreatedAt)
		})
		for _, item := range items {
			sortLevel(item.Children)
		}
	}
	sortLevel(roots)
	return roots
}

// OutlineNodeInput 是「整棵树一次性提交」的入参（AI 候选采纳 / 作者改完再提交）。
// 层级不写死，按嵌套深度推导（第 1 层 = 卷，第 2 层 = 节，第 3 层 = 章）。
type OutlineNodeInput struct {
	Title      string             `json:"title"`
	Summary    string             `json:"summary"`
	Purpose    string             `json:"purpose"`
	Characters []string           `json:"characters"`
	Location   string             `json:"location"`
	Conflict   string             `json:"conflict"`
	Outcome    string             `json:"outcome"`
	Children   []OutlineNodeInput `json:"children"`
}

// FlattenOutlineInput 把嵌套入参摊平成 (level, parentIndex, node) 序列，并做结构校验。
// 返回的节点顺序即写入顺序（父先于子），parentIndex 指向同一批节点中的下标（-1 = 根）。
func FlattenOutlineInput(inputs []OutlineNodeInput) ([]OutlineNode, []int, error) {
	if len(inputs) == 0 {
		return nil, nil, ErrOutlineTreeEmpty
	}
	nodes := make([]OutlineNode, 0, len(inputs))
	parents := make([]int, 0, len(inputs))

	var walk func(items []OutlineNodeInput, level OutlineLevel, parent int) error
	walk = func(items []OutlineNodeInput, level OutlineLevel, parent int) error {
		if len(items) == 0 {
			return nil // 叶子节点（章）没有子节点是正常的，不能因此判成"超过三层"
		}
		if !level.Valid() {
			return ErrOutlineTreeTooDeep
		}
		for _, in := range items {
			node := OutlineNode{
				Level: level, Sequence: len(nodes) + 1,
				Title: in.Title, Summary: in.Summary, Purpose: in.Purpose,
				Characters: in.Characters, Location: in.Location,
				Conflict: in.Conflict, Outcome: in.Outcome,
			}
			node.Normalize()
			if node.Title == "" {
				return fmt.Errorf("%w（%s节点）", ErrOutlineNodeTitleEmpty, level.Name())
			}
			nodes = append(nodes, node)
			parents = append(parents, parent)
			index := len(nodes) - 1
			if err := walk(in.Children, level+1, index); err != nil {
				return err
			}
		}
		return nil
	}

	if err := walk(inputs, OutlineLevelVolume, -1); err != nil {
		return nil, nil, err
	}
	return nodes, parents, nil
}
