package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/service"
)

// 规格书 §38 §49 要求的 AI 能力入口。
//
// 边界：这些接口**只读上下文、只返回结果**，一律不写库（§52 红线）。
// 作者确认后走各自的写入接口（人物 / 世界 / 章节 / 场景 / 大纲），写入路径只有一条。

// ---------- 请求 DTO ----------

type aiRewriteRequest struct {
	ChapterID   string `json:"chapter_id" binding:"required"`
	Text        string `json:"text" binding:"required"`
	Instruction string `json:"instruction"`
}

type aiContinueRequest struct {
	ChapterID   string `json:"chapter_id" binding:"required"`
	Text        string `json:"text"`
	Instruction string `json:"instruction"`
}

type aiGenerateRequest struct {
	Kind        string `json:"kind" binding:"required,oneof=outline character plot scene"`
	WorkID      string `json:"work_id"`
	ChapterID   string `json:"chapter_id"`
	Instruction string `json:"instruction"`
}

type aiChatRequest struct {
	WorkID    string `json:"work_id"`
	ChapterID string `json:"chapter_id"`
	Message   string `json:"message" binding:"required"`
}

type aiAnalyzeRequest struct {
	ChapterID string `json:"chapter_id"`
	Text      string `json:"text"`
	Focus     string `json:"focus"`
}

// ---------- 处理器 ----------

// aiContinue 续写（规格书 §38 §49）。
//
//	@Summary	AI 续写
//	@Tags		ai
//	@Accept		json
//	@Produce	json
//	@Param		body	body		aiContinueRequest	true	"章节与起点文本（text 省略时从本章正文末尾续写）"
//	@Success	200		{object}	Envelope
//	@Router		/ai/continue [post]
func (s *Server) aiContinue(c *gin.Context) {
	if !s.requireAI(c) {
		return
	}
	var req aiContinueRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	text, err := s.writing.ContinueText(c.Request.Context(), s.invoker, service.RewriteInput{
		ChapterID: req.ChapterID, Text: req.Text, Instruction: req.Instruction,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"text": text})
}

// aiExpand 扩写（规格书 §49 单列端点，等价于 rewrite 的「扩写」）。
//
//	@Summary	AI 扩写
//	@Tags		ai
//	@Accept		json
//	@Produce	json
//	@Param		body	body		aiRewriteRequest	true	"章节与选中文本"
//	@Success	200		{object}	Envelope
//	@Router		/ai/expand [post]
func (s *Server) aiExpand(c *gin.Context) {
	if !s.requireAI(c) {
		return
	}
	var req aiRewriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	text, err := s.writing.RewriteText(c.Request.Context(), s.invoker, service.RewriteInput{
		ChapterID: req.ChapterID, Text: req.Text,
		Action: service.RewriteActionExpand, Instruction: req.Instruction,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"text": text})
}

// aiGenerate AI 生成（大纲 / 人物 / 剧情 / 场景候选）。
//
//	@Summary	AI 生成候选（大纲 / 人物 / 剧情 / 场景）
//	@Description	只返回候选，不写库；作者确认后走各自的写入接口。
//	@Tags		ai
//	@Accept		json
//	@Produce	json
//	@Param		body	body		aiGenerateRequest	true	"生成目标与上下文"
//	@Success	200		{object}	Envelope
//	@Router		/ai/generate [post]
func (s *Server) aiGenerate(c *gin.Context) {
	if !s.requireAI(c) {
		return
	}
	var req aiGenerateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	result, err := s.writing.Generate(c.Request.Context(), s.invoker, service.GenerateInput{
		Kind: service.GenerateKind(req.Kind), WorkID: req.WorkID,
		ChapterID: req.ChapterID, Instruction: req.Instruction,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"result": result, "pending_author_review": true})
}

// aiChat AI 助手问答（只回答，不改稿）。
//
//	@Summary	AI 助手问答
//	@Tags		ai
//	@Accept		json
//	@Produce	json
//	@Param		body	body		aiChatRequest	true	"作品/章节与问题"
//	@Success	200		{object}	Envelope
//	@Router		/ai/chat [post]
func (s *Server) aiChat(c *gin.Context) {
	if !s.requireAI(c) {
		return
	}
	var req aiChatRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	reply, err := s.writing.Chat(c.Request.Context(), s.invoker, service.ChatInput{
		WorkID: req.WorkID, ChapterID: req.ChapterID, Message: req.Message,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"reply": reply})
}

// aiAnalyze 就地分析（只给建议，不进问题库）。
//
//	@Summary	AI 就地分析
//	@Description	对选中文本或本章正文做即时分析；与「一致性检查」不同，结果不写入问题库。
//	@Tags		ai
//	@Accept		json
//	@Produce	json
//	@Param		body	body		aiAnalyzeRequest	true	"章节或文本与关注点"
//	@Success	200		{object}	Envelope
//	@Router		/ai/analyze [post]
func (s *Server) aiAnalyze(c *gin.Context) {
	if !s.requireAI(c) {
		return
	}
	var req aiAnalyzeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "请求参数不合法: "+err.Error(), nil)
		return
	}
	result, err := s.writing.AnalyzeText(c.Request.Context(), s.invoker, service.AnalyzeTextInput{
		ChapterID: req.ChapterID, Text: req.Text, Focus: req.Focus,
	})
	if err != nil {
		s.failFromError(c, err)
		return
	}
	OK(c, gin.H{"result": result})
}

// requireAI 统一检查「服务就绪 + 模型已配置」。
func (s *Server) requireAI(c *gin.Context) bool {
	if !s.requireServices(c) {
		return false
	}
	if s.invoker == nil {
		Fail(c, http.StatusServiceUnavailable, "MODEL_NOT_READY", "模型未配置：请先在「模型设置」页配置并启用一个模型", nil)
		return false
	}
	return true
}
