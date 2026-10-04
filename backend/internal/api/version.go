package api

import (
	"context"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
)

// ---------- 响应 DTO ----------

// EntityVersionResponse 是版本条目（列表不含 payload）。
type EntityVersionResponse struct {
	ID             string         `json:"id"`
	EntityType     string         `json:"entity_type"`
	EntityID       string         `json:"entity_id"`
	CreativeWorkID string         `json:"creative_work_id"`
	VersionNo      int            `json:"version_no"`
	Payload        map[string]any `json:"payload,omitempty"`
	Note           string         `json:"note"`
	CreatedAt      time.Time      `json:"created_at"`
}

func toVersionResponse(v domain.EntityVersion, withPayload bool) EntityVersionResponse {
	out := EntityVersionResponse{
		ID: v.ID, EntityType: string(v.EntityType), EntityID: v.EntityID,
		CreativeWorkID: v.CreativeWorkID, VersionNo: v.VersionNo,
		Note: v.Note, CreatedAt: v.CreatedAt,
	}
	if withPayload {
		out.Payload = v.Payload
	}
	return out
}

// ---------- 通用辅助 ----------

// snapshotCharacter / World / Outline 都是「尽力而为」：快照失败只记日志，
// 不能让一次正常保存因为版本记录出错而失败。
func (s *Server) snapshotCharacter(ctx context.Context, characterID, note string) {
	if s.versions == nil || characterID == "" {
		return
	}
	if _, err := s.versions.SnapshotCharacter(ctx, characterID, note); err != nil {
		s.logger.Warn("人物版本快照失败", "character_id", characterID, "error", err)
	}
}

func (s *Server) snapshotWorld(ctx context.Context, workID, note string) {
	if s.versions == nil || workID == "" {
		return
	}
	if _, err := s.versions.SnapshotWorld(ctx, workID, note); err != nil {
		s.logger.Warn("世界观版本快照失败", "creative_work_id", workID, "error", err)
	}
}

func (s *Server) snapshotOutline(ctx context.Context, workID, note string) {
	if s.versions == nil || workID == "" {
		return
	}
	if _, err := s.versions.SnapshotOutline(ctx, workID, note); err != nil {
		s.logger.Warn("大纲版本快照失败", "creative_work_id", workID, "error", err)
	}
}

func (s *Server) snapshotWorldByRule(ctx context.Context, ruleID, note string) {
	if s.versions == nil || s.creative == nil || ruleID == "" {
		return
	}
	workID, err := s.creative.WorkIDByWorldRule(ctx, ruleID)
	if err != nil {
		s.logger.Warn("规则版本快照取作品失败", "rule_id", ruleID, "error", err)
		return
	}
	s.snapshotWorld(ctx, workID, note)
}

// snapshotError 把版本服务错误翻译成 HTTP 响应。
func (s *Server) failVersion(c *gin.Context, err error) {
	s.failFromError(c, err)
}

// ---------- 人物版本 ----------

// listCharacterVersions 人物版本列表。
//
//	@Summary	人物版本列表
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创人物 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative-characters/{id}/versions [get]
func (s *Server) listCharacterVersions(c *gin.Context) {
	s.listVersions(c, string(domain.VersionCreativeCharacter), c.Param("id"))
}

// snapshotCharacterHandler 手动给人物存一版。
//
//	@Summary	人物存档（手动）
//	@Tags		versions
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string					true	"二创人物 ID"
//	@Param		body	body		versionNoteRequest		false	"备注"
//	@Success	201		{object}	Envelope
//	@Router		/creative-characters/{id}/versions [post]
func (s *Server) snapshotCharacterHandler(c *gin.Context) {
	if !s.requireServices(c) || !s.requireVersions(c) {
		return
	}
	var req versionNoteRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	v, err := s.versions.SnapshotCharacter(c.Request.Context(), c.Param("id"), req.Note)
	if err != nil {
		s.failVersion(c, err)
		return
	}
	if v == nil {
		OK(c, gin.H{"created": false, "reason": "与最新一版相同，未新建版本"})
		return
	}
	Created(c, toVersionResponse(*v, false))
}

// getCharacterVersion 人物某个版本（含快照内容）。
//
//	@Summary	人物版本详情
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创人物 ID"
//	@Param		no	path		int		true	"版本号"
//	@Success	200	{object}	Envelope
//	@Router		/creative-characters/{id}/versions/{no} [get]
func (s *Server) getCharacterVersion(c *gin.Context) {
	s.getVersion(c, string(domain.VersionCreativeCharacter), c.Param("id"))
}

// restoreCharacterVersion 恢复人物到某个版本。
//
//	@Summary	恢复人物版本
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创人物 ID"
//	@Param		no	path		int		true	"版本号"
//	@Success	200	{object}	Envelope
//	@Router		/creative-characters/{id}/versions/{no}/restore [post]
func (s *Server) restoreCharacterVersion(c *gin.Context) {
	s.restoreVersion(c, string(domain.VersionCreativeCharacter), c.Param("id"))
}

// ---------- 世界观版本 ----------

// listWorldVersions 世界观版本列表。
//
//	@Summary	世界观版本列表
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/world/versions [get]
func (s *Server) listWorldVersions(c *gin.Context) {
	s.listVersions(c, string(domain.VersionCreativeWorld), c.Param("id"))
}

// snapshotWorldHandler 手动给世界观存一版。
//
//	@Summary	世界观存档（手动）
//	@Tags		versions
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"二创作品 ID"
//	@Param		body	body		versionNoteRequest	false	"备注"
//	@Success	201		{object}	Envelope
//	@Router		/creative/{id}/world/versions [post]
func (s *Server) snapshotWorldHandler(c *gin.Context) {
	if !s.requireServices(c) || !s.requireVersions(c) {
		return
	}
	var req versionNoteRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	v, err := s.versions.SnapshotWorld(c.Request.Context(), c.Param("id"), req.Note)
	if err != nil {
		s.failVersion(c, err)
		return
	}
	if v == nil {
		OK(c, gin.H{"created": false, "reason": "与最新一版相同，未新建版本"})
		return
	}
	Created(c, toVersionResponse(*v, false))
}

// getWorldVersion 世界观某个版本。
//
//	@Summary	世界观版本详情
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Param		no	path		int		true	"版本号"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/world/versions/{no} [get]
func (s *Server) getWorldVersion(c *gin.Context) {
	s.getVersion(c, string(domain.VersionCreativeWorld), c.Param("id"))
}

// restoreWorldVersion 恢复世界观到某个版本。
//
//	@Summary	恢复世界观版本
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Param		no	path		int		true	"版本号"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/world/versions/{no}/restore [post]
func (s *Server) restoreWorldVersion(c *gin.Context) {
	s.restoreVersion(c, string(domain.VersionCreativeWorld), c.Param("id"))
}

// ---------- 大纲版本 ----------

// listOutlineVersions 大纲版本列表。
//
//	@Summary	大纲版本列表
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/outline/versions [get]
func (s *Server) listOutlineVersions(c *gin.Context) {
	s.listVersions(c, string(domain.VersionCreativeOutline), c.Param("id"))
}

// snapshotOutlineHandler 手动给大纲存一版。
//
//	@Summary	大纲存档（手动）
//	@Tags		versions
//	@Accept		json
//	@Produce	json
//	@Param		id		path		string				true	"二创作品 ID"
//	@Param		body	body		versionNoteRequest	false	"备注"
//	@Success	201		{object}	Envelope
//	@Router		/creative/{id}/outline/versions [post]
func (s *Server) snapshotOutlineHandler(c *gin.Context) {
	if !s.requireServices(c) || !s.requireVersions(c) {
		return
	}
	var req versionNoteRequest
	if err := bindOptionalJSON(c, &req); err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, err.Error(), nil)
		return
	}
	v, err := s.versions.SnapshotOutline(c.Request.Context(), c.Param("id"), req.Note)
	if err != nil {
		s.failVersion(c, err)
		return
	}
	if v == nil {
		OK(c, gin.H{"created": false, "reason": "与最新一版相同，未新建版本"})
		return
	}
	Created(c, toVersionResponse(*v, false))
}

// getOutlineVersion 大纲某个版本。
//
//	@Summary	大纲版本详情
//	@Tags		versions
//	@Produce	json
//	@Param		id	path		string	true	"二创作品 ID"
//	@Param		no	path		int		true	"版本号"
//	@Success	200	{object}	Envelope
//	@Router		/creative/{id}/outline/versions/{no} [get]
func (s *Server) getOutlineVersion(c *gin.Context) {
	s.getVersion(c, string(domain.VersionCreativeOutline), c.Param("id"))
}

// restoreOutlineVersion 恢复大纲到某个版本。
//
//	@Summary		恢复大纲版本
//	@Description	只回填快照里记录到的章节大纲字段；快照之后新建的章节/卷不会被删除。
//	@Tags			versions
//	@Produce		json
//	@Param			id	path		string	true	"二创作品 ID"
//	@Param			no	path		int		true	"版本号"
//	@Success		200	{object}	Envelope
//	@Router			/creative/{id}/outline/versions/{no}/restore [post]
func (s *Server) restoreOutlineVersion(c *gin.Context) {
	s.restoreVersion(c, string(domain.VersionCreativeOutline), c.Param("id"))
}

// ---------- 公共实现 ----------

type versionNoteRequest struct {
	Note string `json:"note" example:"定稿前存档"`
}

func (s *Server) requireVersions(c *gin.Context) bool {
	if s.versions == nil {
		Fail(c, http.StatusServiceUnavailable, "SERVICE_UNAVAILABLE", "版本服务未启用", nil)
		return false
	}
	return true
}

func (s *Server) listVersions(c *gin.Context, entityType, entityID string) {
	if !s.requireServices(c) || !s.requireVersions(c) {
		return
	}
	items, err := s.versions.List(c.Request.Context(), entityType, entityID)
	if err != nil {
		s.failVersion(c, err)
		return
	}
	resp := make([]EntityVersionResponse, 0, len(items))
	for _, v := range items {
		resp = append(resp, toVersionResponse(v, false))
	}
	OK(c, gin.H{"items": resp, "total": len(resp)})
}

func (s *Server) getVersion(c *gin.Context, entityType, entityID string) {
	if !s.requireServices(c) || !s.requireVersions(c) {
		return
	}
	no, err := parseIntParam(c, "no")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "版本号必须是整数", nil)
		return
	}
	version, err := s.versions.Get(c.Request.Context(), entityType, entityID, no)
	if err != nil {
		s.failVersion(c, err)
		return
	}
	OK(c, toVersionResponse(*version, true))
}

func (s *Server) restoreVersion(c *gin.Context, entityType, entityID string) {
	if !s.requireServices(c) || !s.requireVersions(c) {
		return
	}
	no, err := parseIntParam(c, "no")
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "版本号必须是整数", nil)
		return
	}
	if err := s.versions.Restore(c.Request.Context(), entityType, entityID, no); err != nil {
		s.failVersion(c, err)
		return
	}
	OK(c, gin.H{"restored": true, "version_no": no})
}
