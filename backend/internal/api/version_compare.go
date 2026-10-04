package api

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// 版本比较（规格书 §59：版本管理要支持「查看 / 恢复 / 比较」）。
//
// 用统一入口而不是给四类实体各加一条 compare 路由，原因有二：
//  1. 四类比较的输入完全一致（实体类型 + 实体 ID + 两个版本号），语义上是同一个操作；
//  2. gin 的路由树里 /:id/versions/:no 与 /:id/versions/compare 会在同一层出现
//     「静态段 vs 参数段」冲突，容易误伤既有路由。

// compareVersions 比较同一实体的两个版本（0 表示「当前状态」）。
//
//	@Summary	版本比较
//	@Tags		versions
//	@Produce	json
//	@Param		entity_type	query	string	true	"实体类型：chapter / creative_character / creative_world / creative_outline"
//	@Param		entity_id	query	string	true	"实体 ID（章节 ID / 人物 ID / 二创作品 ID）"
//	@Param		from		query	int		true	"基准版本号（0 = 当前状态）"
//	@Param		to			query	int		true	"对比版本号（0 = 当前状态）"
//	@Success	200			{object}	Envelope
//	@Router		/versions/compare [get]
func (s *Server) compareVersions(c *gin.Context) {
	if !s.requireServices(c) || !s.requireVersions(c) {
		return
	}

	entityType := strings.TrimSpace(c.Query("entity_type"))
	entityID := strings.TrimSpace(c.Query("entity_id"))
	if entityType == "" || entityID == "" {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "entity_type 与 entity_id 必填", nil)
		return
	}

	from, err := parseOptionalVersionNo(c.Query("from"))
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "from 必须是非负整数", nil)
		return
	}
	to, err := parseOptionalVersionNo(c.Query("to"))
	if err != nil {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "to 必须是非负整数", nil)
		return
	}
	if from == 0 && to == 0 {
		Fail(c, http.StatusBadRequest, CodeBadRequest, "from 与 to 不能同时为 0（都是当前状态就没有可比较的）", nil)
		return
	}

	ctx := c.Request.Context()
	var result any
	if entityType == "chapter" {
		result, err = s.versions.CompareChapter(ctx, entityID, from, to)
	} else {
		result, err = s.versions.Compare(ctx, entityType, entityID, from, to)
	}
	if err != nil {
		s.failVersion(c, err)
		return
	}
	OK(c, result)
}

// parseOptionalVersionNo 解析版本号；空字符串按 0（当前状态）处理。
func parseOptionalVersionNo(raw string) (int, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return 0, nil
	}
	no, err := strconv.Atoi(raw)
	if err != nil || no < 0 {
		return 0, strconv.ErrSyntax
	}
	return no, nil
}
