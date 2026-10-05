package service

import (
	"context"
	"fmt"
	"strings"

	"github.com/yanaoyi/novamindv2/backend/internal/domain"
	"github.com/yanaoyi/novamindv2/backend/internal/repository"
	"github.com/yanaoyi/novamindv2/backend/internal/retrieval"
)

// RetrievalService 是检索入口（Phase 9 §9.1.5）。
//
// 当前是单用户模式：所有分块都挂在默认账号名下，所以检索时用 DefaultUserID 作为 owner。
// 将来接入登录后，这里改成"当前登录用户"即可，检索与隔离逻辑不用动。
type RetrievalService struct {
	source retrieval.ChunkSource
	users  *repository.UserRepo
}

// NewRetrievalService 构建服务。
func NewRetrievalService(source retrieval.ChunkSource, users *repository.UserRepo) *RetrievalService {
	return &RetrievalService{source: source, users: users}
}

// Search 在指定作品里检索。
func (s *RetrievalService) Search(
	ctx context.Context,
	workKind, workID, query string,
	topK int,
) ([]retrieval.ScoredChunk, error) {
	workKind = strings.TrimSpace(workKind)
	if workKind != domain.WorkKindOriginal && workKind != domain.WorkKindCreative {
		return nil, fmt.Errorf("%w：work_kind 只能是 original 或 creative", ErrBadRequest)
	}
	if strings.TrimSpace(workID) == "" {
		return nil, fmt.Errorf("%w：work_id 必填", ErrBadRequest)
	}
	if strings.TrimSpace(query) == "" {
		return nil, fmt.Errorf("%w：query 必填", ErrBadRequest)
	}
	ownerID, err := s.users.DefaultUserID(ctx)
	if err != nil {
		return nil, err
	}
	return retrieval.Search(ctx, s.source, ownerID, workKind, workID, query, topK)
}
