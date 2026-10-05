// Package prompts 把 Prompt 模板编译进二进制。
//
// 为什么放在 backend 模块内：go:embed 只能嵌入当前包目录下的文件，
// 而 Go 模块根在 backend/。放在模块外就无法随二进制分发，部署时得额外同步目录。
// 约定文件名格式：<name>.<version>.md，例如 character/character_extract.v1.md
package prompts

import "embed"

// FS 包含全部 Prompt 模板与 JSON Schema。
//
//go:embed original/*.md character/*.md world/*.md plot/*.md outline/*.md writing/*.md review/*.md memory/*.md schemas/*.json
var FS embed.FS
