package service

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// 一致性检查的上下文组装（规格书 §31 优先级 + §39 五类检查）。
//
// 之前只送了人物与世界观，时间线是一句固定占位的话，剧情与原著继承根本没进 Prompt ——
// 等于四类检查里只有两类是真的。这里按 §39 把五类上下文都装配齐：
//
//	人物一致性   ← 二创人物（含 DNA 权重）
//	世界一致性   ← 二创世界 + 世界规则
//	时间线一致性 ← 二创时间线（按 sequence 排序，带继承状态与时间标签）
//	剧情一致性   ← 章节大纲链（章 → 摘要/冲突/目的/结果），即二创剧情骨架
//	原著继承一致性 ← 原著↔二创映射 + 继承状态，用于判断是否偏离"继承自原著的设定"

// ConsistencyContext 是喂给一致性检查的两类核心上下文。
type ConsistencyContext struct {
	Characters  string
	World       string
	Timeline    string
	Plot        string
	Inheritance string
}

// maxConsistencyContextItems 限制每类上下文的条目数，避免把 Prompt 撑爆。
const maxConsistencyContextItems = 60

// BuildConsistencyContext 组装五类上下文；任何一类取不到就如实留空，不编造。
func (s *WritingService) BuildConsistencyContext(ctx context.Context, workID string) ConsistencyContext {
	out := ConsistencyContext{}
	if s.ctxReader == nil {
		return out
	}

	if characters, err := s.ctxReader.ListCharacters(ctx, workID); err == nil {
		var sb strings.Builder
		for i, c := range characters {
			if i >= maxConsistencyContextItems {
				break
			}
			dims := make([]string, 0, 4)
			for name, dim := range c.DNA.Dimensions() {
				if dim.Weight > 0 && strings.TrimSpace(dim.Text) != "" {
					dims = append(dims, fmt.Sprintf("%s(%d%%)=%s", name, dim.Weight, dim.Text))
				}
			}
			sort.Strings(dims)
			fmt.Fprintf(&sb, "- %s（%s）：%s %s\n", c.Name, c.SourceType, c.Description, strings.Join(dims, " "))
		}
		out.Characters = strings.TrimSpace(sb.String())
	}

	if detail, err := s.ctxReader.GetWorldDetail(ctx, workID); err == nil && detail.World != nil {
		var sb strings.Builder
		fmt.Fprintf(&sb, "世界：%s（继承模式 %s）%s\n", detail.World.Name, detail.World.InheritanceMode, detail.World.Description)
		for i, rule := range detail.Rules {
			if i >= maxConsistencyContextItems {
				break
			}
			if rule.Status == domain.RuleRemoved {
				continue
			}
			fmt.Fprintf(&sb, "- [%s][%s] %s：%s\n", rule.Status, rule.Category, rule.Name, rule.Description)
		}
		out.World = strings.TrimSpace(sb.String())
	}

	if events, err := s.ctxReader.GetTimeline(ctx, workID); err == nil && len(events) > 0 {
		ordered := make([]domain.CreativeTimelineEvent, len(events))
		copy(ordered, events)
		sort.SliceStable(ordered, func(i, j int) bool { return ordered[i].Sequence < ordered[j].Sequence })

		var sb strings.Builder
		for i, e := range ordered {
			if i >= maxConsistencyContextItems {
				break
			}
			if e.Status == domain.TimelineRemoved {
				continue
			}
			label := strings.TrimSpace(e.TimeLabel)
			if label == "" {
				label = fmt.Sprintf("第 %d 位", e.Sequence)
			}
			fmt.Fprintf(&sb, "- [%s] %s：%s %s\n", e.Status, label, e.Title, e.Description)
		}
		out.Timeline = strings.TrimSpace(sb.String())
	}

	// 剧情骨架用章节大纲链表达：二创剧情模型（§26）落地前，这是作者能掌控的剧情事实来源
	if chapters, err := s.repo.ListChapters(ctx, workID, false); err == nil && len(chapters) > 0 {
		var sb strings.Builder
		for i, c := range chapters {
			if i >= maxConsistencyContextItems {
				break
			}
			parts := make([]string, 0, 4)
			if v := strings.TrimSpace(c.Summary); v != "" {
				parts = append(parts, "摘要："+v)
			}
			if v := strings.TrimSpace(c.Conflict); v != "" {
				parts = append(parts, "冲突："+v)
			}
			if v := strings.TrimSpace(c.Purpose); v != "" {
				parts = append(parts, "目的："+v)
			}
			if v := strings.TrimSpace(c.Outcome); v != "" {
				parts = append(parts, "结果："+v)
			}
			if len(parts) == 0 {
				continue
			}
			fmt.Fprintf(&sb, "- 第 %d 章 %s：%s\n", c.ChapterNo, c.Title, strings.Join(parts, "；"))
		}
		out.Plot = strings.TrimSpace(sb.String())
	}

	if mappings, err := s.ctxReader.ListMappings(ctx, workID); err == nil && len(mappings) > 0 {
		var sb strings.Builder
		for i, m := range mappings {
			if i >= maxConsistencyContextItems {
				break
			}
			desc := strings.TrimSpace(m.Description)
			if desc != "" {
				desc = "（" + desc + "）"
			}
			fmt.Fprintf(&sb, "- 原著 %s %s → 二创 %s %s：%s%s\n",
				m.OriginalType, m.OriginalID, m.CreativeType, m.CreativeID, m.MappingType, desc)
		}
		out.Inheritance = strings.TrimSpace(sb.String())
	}

	return out
}
