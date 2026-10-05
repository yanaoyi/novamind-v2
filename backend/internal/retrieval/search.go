package retrieval

import (
	"context"
	"math"
	"sort"
	"strings"
	"unicode"
)

// IndexedChunk 是参与检索的一条已索引分块（由仓储提供）。
type IndexedChunk struct {
	ID      string
	RefKind string
	RefID   *string
	Seq     int
	Content string
}

// ChunkSource 提供某作品的全部分块（BM25 路需要全量打分）。
type ChunkSource interface {
	ListChunks(ctx context.Context, ownerUserID, workKind, workID string) ([]IndexedChunk, error)
}

// ScoredChunk 是检索结果。
type ScoredChunk struct {
	IndexedChunk
	Score float64
	// MatchFrom 记录命中来自哪几路（当前只有 bm25；将来接上向量路会多一个 "vector"）
	MatchFrom []string
}

const (
	bm25K1 = 1.2
	bm25B  = 0.75
	// rrfK 是 RRF 融合常数（任务书指定 60）：score = sum 1/(k + rank)
	rrfK = 60.0
	// DefaultTopK 是默认返回条数（任务书：topK 默认 8）
	DefaultTopK = 8
)

// Search 检索某个作品里与 query 最相关的分块。
//
// 当前只有 BM25 一路（char-bigram，零依赖）；RRF 融合已就位，
// 等 pgvector 可用时把向量路排名一并传进 FuseRRF 即可，调用方不必改。
func Search(
	ctx context.Context,
	source ChunkSource,
	ownerUserID, workKind, workID, query string,
	topK int,
) ([]ScoredChunk, error) {
	if topK <= 0 {
		topK = DefaultTopK
	}
	terms := tokenize(query)
	if len(terms) == 0 {
		return []ScoredChunk{}, nil
	}
	chunks, err := source.ListChunks(ctx, ownerUserID, workKind, workID)
	if err != nil {
		return nil, err
	}
	if len(chunks) == 0 {
		return []ScoredChunk{}, nil
	}
	ranked := bm25Rank(chunks, terms, topK*2)
	fused := FuseRRF(ranked)
	if len(fused) > topK {
		fused = fused[:topK]
	}
	return fused, nil
}

// bm25Rank 用 BM25 给全部块打分，按分数倒序取前 limit 条（0 分不返回）。
func bm25Rank(chunks []IndexedChunk, terms []string, limit int) []ScoredChunk {
	docs := make([]map[string]int, len(chunks))
	df := map[string]int{}
	totalLen := 0
	for i, c := range chunks {
		tokens := tokenize(c.Content)
		tf := make(map[string]int, len(tokens))
		for _, t := range tokens {
			tf[t]++
		}
		docs[i] = tf
		totalLen += len(tokens)
		for t := range tf {
			df[t]++
		}
	}
	avgdl := float64(totalLen) / float64(len(chunks))
	if avgdl == 0 {
		avgdl = 1
	}

	scored := make([]ScoredChunk, 0, len(chunks))
	for i, c := range chunks {
		dl := 0
		for _, n := range docs[i] {
			dl += n
		}
		score := 0.0
		for _, term := range terms {
			f := float64(docs[i][term])
			if f == 0 {
				continue
			}
			n := float64(len(chunks))
			idf := math.Log(1 + (n-float64(df[term])+0.5)/(float64(df[term])+0.5))
			denom := f + bm25K1*(1-bm25B+bm25B*float64(dl)/avgdl)
			score += idf * f * (bm25K1 + 1) / denom
		}
		if score <= 0 {
			continue
		}
		scored = append(scored, ScoredChunk{IndexedChunk: c, Score: score, MatchFrom: []string{"bm25"}})
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].Score > scored[j].Score })
	if len(scored) > limit {
		scored = scored[:limit]
	}
	return scored
}

// FuseRRF 用 Reciprocal Rank Fusion 融合多路排名（当前只有一路，接口先就位）。
//
// 同一块在多路都命中时分数相加（1/(k+rank1) + 1/(k+rank2)）——
// 这正是混合检索要的效果：词面命中与语义命中互相加成。
func FuseRRF(ranked ...[]ScoredChunk) []ScoredChunk {
	type merged struct {
		chunk ScoredChunk
		score float64
		from  []string
	}
	byID := map[string]*merged{}
	var order []string
	for _, list := range ranked {
		for rank, item := range list {
			m, ok := byID[item.ID]
			if !ok {
				copied := item
				copied.Score = 0
				m = &merged{chunk: copied}
				byID[item.ID] = m
				order = append(order, item.ID)
			}
			m.score += 1.0 / (rrfK + float64(rank) + 1)
			for _, src := range item.MatchFrom {
				if !contains(m.from, src) {
					m.from = append(m.from, src)
				}
			}
		}
	}
	out := make([]ScoredChunk, 0, len(order))
	for _, id := range order {
		m := byID[id]
		m.chunk.Score = m.score
		m.chunk.MatchFrom = m.from
		out = append(out, m.chunk)
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Score > out[j].Score })
	return out
}

// tokenize 是中文 char-bigram 分词（任务书指定的零依赖方案）。
//
// "长篇记忆" → ["长篇","篇记","记忆"]；单字词（如"剑"）也保留，
// 否则单字查询会全部落空。标点与空白当分隔符。
func tokenize(s string) []string {
	fields := strings.FieldsFunc(strings.ToLower(s), func(r rune) bool {
		return !unicode.IsLetter(r) && !unicode.IsNumber(r)
	})
	var out []string
	for _, f := range fields {
		runes := []rune(f)
		if len(runes) == 1 {
			out = append(out, string(runes))
			continue
		}
		for i := 0; i+1 < len(runes); i++ {
			out = append(out, string(runes[i:i+2]))
		}
	}
	return out
}

func contains(list []string, want string) bool {
	for _, item := range list {
		if item == want {
			return true
		}
	}
	return false
}
