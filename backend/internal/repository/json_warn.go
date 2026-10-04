package repository

import (
	"encoding/json"
	"log/slog"
)

// unmarshalJSONB 解析 JSONB 列，失败时记一条 warn 而不是静默置空。
//
// 为什么保留"继续返回"而不是直接报错（审查 P2 的建议是返回错误）：
// 这些列都是辅助信息（人物数组、融合说明、修改记录），
// 一份历史数据里某个 JSONB 坏了就让整章/整个人物打不开，代价过大；
// 但静默置空会让作者以为"我填过的东西没了"。记日志能定位，界面仍可用 —— 这是刻意的取舍。
func unmarshalJSONB(table, column, raw string, dst any) {
	if raw == "" {
		return
	}
	if err := json.Unmarshal([]byte(raw), dst); err != nil {
		slog.Warn("JSONB 字段解析失败（已按空值处理）",
			slog.String("table", table), slog.String("column", column),
			slog.String("error", err.Error()))
	}
}
