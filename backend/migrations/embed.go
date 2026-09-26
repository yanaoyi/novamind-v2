// Package migrations 通过 embed 把 SQL 迁移文件打进二进制，
// 使 cmd/migrate 在任何工作目录下都能找到迁移文件。
package migrations

import "embed"

// FS 包含全部 .sql 迁移文件。
//
//go:embed *.sql
var FS embed.FS
