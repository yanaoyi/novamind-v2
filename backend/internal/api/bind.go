package api

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

// bindOptionalJSON 解析"可有可无"的 JSON 请求体：
//   - 空 body（Content-Length 0）是合法的：调用方只想用一个带默认值的请求；
//   - 非空但非法 → 报错，交给调用方返回 400。
//
// 为什么不能直接 `_ = c.ShouldBindJSON(&req)`（审查 P2）：
// 非法 JSON 会被静默当成零值处理，例如 generateChapter 会以 target_words=0 入队，
// 使用者以为自己传了参数，实际任务按默认值跑，排查时完全看不出问题。
func bindOptionalJSON(c *gin.Context, dst any) error {
	if c.Request == nil || c.Request.Body == nil || c.Request.ContentLength == 0 {
		return nil
	}
	if err := c.ShouldBindJSON(dst); err != nil {
		return fmt.Errorf("请求体不是合法 JSON: %w", err)
	}
	return nil
}
