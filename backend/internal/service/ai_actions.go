package service

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// 规格书 §38 §49 要求的 AI 能力：续写 / 扩写 / 生成 / 问答 / 就地分析。
//
// 边界（§52 红线）：这里的「生成」只把结果返回给作者，**不落库**；
// 作者确认后再走既有接口写入，所有写入路径仍然只有一条。

// ContinueText 续写：text 为空时从本章正文末尾接着写。
func (s *WritingService) ContinueText(ctx context.Context, runner PromptRunner, in RewriteInput) (string, error) {
	if strings.TrimSpace(in.Text) == "" {
		chapter, err := s.repo.GetChapter(ctx, in.ChapterID)
		if err != nil {
			return "", err
		}
		content := strings.TrimSpace(chapter.Content)
		if content == "" {
			return "", errors.New("本章还没有正文：先用「让 AI 写本章」生成草稿，或手写一段再续写")
		}
		in.Text = tailRunes(content, 1200) // 只取末尾一段作起点，不把整章塞进 Prompt
	}
	in.Action = RewriteActionContinue
	return s.RewriteText(ctx, runner, in)
}

// ChatInput 是 AI 助手问答入参（只回答，不改稿）。
type ChatInput struct {
	WorkID    string
	ChapterID string
	Message   string
}

// Chat 带设定的问答（规格书 §34 的最简形态：能读设定、不动数据）。
func (s *WritingService) Chat(ctx context.Context, runner PromptRunner, in ChatInput) (string, error) {
	if strings.TrimSpace(in.Message) == "" {
		return "", errors.New("问题不能为空")
	}
	workID := in.WorkID
	chapterGoal := ""
	if in.ChapterID != "" {
		chapterCtx, err := s.BuildContext(ctx, in.ChapterID)
		if err != nil {
			return "", err
		}
		chapter, err := s.repo.GetChapter(ctx, in.ChapterID)
		if err != nil {
			return "", err
		}
		workID = chapter.CreativeWorkID
		chapterGoal = chapterCtx.ChapterGoal
	}
	if workID == "" {
		return "", errors.New("需要 work_id 或 chapter_id")
	}
	if _, err := s.creative.GetWorkByID(ctx, workID); err != nil {
		return "", err
	}

	cctx := s.BuildConsistencyContext(ctx, workID)
	return runner.RunPrompt(ctx, "ai_chat", map[string]any{
		"Message":          in.Message,
		"ChapterGoal":      chapterGoal,
		"CharacterContext": cctx.Characters,
		"WorldContext":     cctx.World,
		"TimelineContext":  cctx.Timeline,
		"PlotContext":      cctx.Plot,
	})
}

// AnalyzeTextInput 是「就地分析」入参（只给建议，不改稿、不进问题库）。
type AnalyzeTextInput struct {
	ChapterID string
	Text      string
	Focus     string
}

// AnalyzeText 对选中文本或本章正文做一次即时分析（规格书 §38）。
func (s *WritingService) AnalyzeText(ctx context.Context, runner PromptRunner, in AnalyzeTextInput) (map[string]any, error) {
	text := strings.TrimSpace(in.Text)
	cctx := ConsistencyContext{}

	if in.ChapterID != "" {
		chapter, err := s.repo.GetChapter(ctx, in.ChapterID)
		if err != nil {
			return nil, err
		}
		if text == "" {
			text = chapter.Content
		}
		cctx = s.BuildConsistencyContext(ctx, chapter.CreativeWorkID)
	}
	if strings.TrimSpace(text) == "" {
		return nil, errors.New("没有可分析的文本")
	}

	reply, err := runner.RunPrompt(ctx, "text_analyze", map[string]any{
		"Focus":            in.Focus,
		"Text":             trimChars(text, maxAnalysisChars),
		"CharacterContext": cctx.Characters,
		"WorldContext":     cctx.World,
	})
	if err != nil {
		return nil, err
	}
	obj, err := ExtractJSONObject(reply)
	if err != nil {
		return nil, fmt.Errorf("模型输出不是合法 JSON：%w", err)
	}
	return obj, nil
}

// GenerateKind 是「AI 生成」的目标类型（规格书 §38：大纲 / 人物 / 剧情 / 场景）。
type GenerateKind string

const (
	GenerateOutline   GenerateKind = "outline"
	GenerateCharacter GenerateKind = "character"
	GeneratePlot      GenerateKind = "plot"
	GenerateScene     GenerateKind = "scene"
)

// Valid 判断生成目标是否合法。
func (k GenerateKind) Valid() bool {
	switch k {
	case GenerateOutline, GenerateCharacter, GeneratePlot, GenerateScene:
		return true
	default:
		return false
	}
}

// promptFor 返回该生成目标使用的模板名。
func (k GenerateKind) promptFor() string {
	switch k {
	case GenerateOutline:
		return "outline_generate"
	case GenerateCharacter:
		return "character_generate"
	case GeneratePlot:
		return "plot_generate"
	case GenerateScene:
		return "scene_generate"
	default:
		return ""
	}
}

// GenerateInput 是 AI 生成入参。
type GenerateInput struct {
	Kind        GenerateKind
	WorkID      string
	ChapterID   string
	Instruction string
}

// Generate 生成大纲/人物/剧情/场景候选，**只返回给作者确认，不落库**（§52 红线）。
func (s *WritingService) Generate(ctx context.Context, runner PromptRunner, in GenerateInput) (map[string]any, error) {
	if !in.Kind.Valid() {
		return nil, fmt.Errorf("不支持的生成目标：%s", in.Kind)
	}
	workID := in.WorkID
	if workID == "" && in.ChapterID != "" {
		chapter, err := s.repo.GetChapter(ctx, in.ChapterID)
		if err != nil {
			return nil, err
		}
		workID = chapter.CreativeWorkID
	}
	if workID == "" {
		return nil, errors.New("需要 work_id 或 chapter_id")
	}
	work, err := s.creative.GetWorkByID(ctx, workID)
	if err != nil {
		return nil, err
	}

	cctx := s.BuildConsistencyContext(ctx, workID)
	vars := map[string]any{
		"WorkTitle":        work.Title,
		"Premise":          work.Description,
		"Instruction":      in.Instruction,
		"CharacterContext": cctx.Characters,
		"WorldContext":     cctx.World,
		"TimelineContext":  cctx.Timeline,
		"PlotContext":      cctx.Plot,
	}
	// 场景生成需要具体章节的信息
	if in.Kind == GenerateScene && in.ChapterID != "" {
		chapter, err := s.repo.GetChapter(ctx, in.ChapterID)
		if err != nil {
			return nil, err
		}
		vars["ChapterTitle"] = chapter.Title
		vars["ChapterSummary"] = chapter.Summary
		vars["ChapterConflict"] = chapter.Conflict
		vars["ChapterPurpose"] = chapter.Purpose
	} else {
		vars["ChapterTitle"] = ""
		vars["ChapterSummary"] = ""
		vars["ChapterConflict"] = ""
		vars["ChapterPurpose"] = ""
	}

	reply, err := runner.RunPrompt(ctx, in.Kind.promptFor(), vars)
	if err != nil {
		return nil, err
	}
	obj, err := ExtractJSONObject(reply)
	if err != nil {
		return nil, fmt.Errorf("模型输出不是合法 JSON：%w", err)
	}
	// 生成结果一律标记为待作者确认；前端负责让作者改完再走写入接口
	obj["_pending_author_review"] = true
	return obj, nil
}

func tailRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[len(r)-n:])
}
