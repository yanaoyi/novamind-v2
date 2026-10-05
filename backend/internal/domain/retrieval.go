package domain

// RetrievalChunk 是一个待入库的检索分块（Phase 9 §9.1）。
type RetrievalChunk struct {
	Seq        int
	Content    string
	TokenCount int
}

// 检索分块来源类型（与迁移 0019 的 CHECK 约束一致）。
const (
	ChunkRefChapter        = "chapter"
	ChunkRefOutlineNode    = "outline_node"
	ChunkRefWorldRule      = "world_rule"
	ChunkRefCharacter      = "character"
	ChunkRefEvent          = "event"
	ChunkRefMemoryFact     = "memory_fact"
	ChunkRefChapterSummary = "chapter_summary"
)

// 作品归属：原著 / 二创（与迁移 0019 的 CHECK 约束一致）。
const (
	WorkKindOriginal = "original"
	WorkKindCreative = "creative"
)
