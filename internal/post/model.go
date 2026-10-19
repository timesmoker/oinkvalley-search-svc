// Package post — 색인된 글 문서와 색인 설정 (indexer·browse·batch 가 같이 쓰는 스키마).
package post

import (
	"github.com/oinkvalley/search/internal/meili"
	"github.com/timesmoker/oinkvalley-protos/go/board/boardv1"
)

// Doc — 색인 문서. 가시성 필드는 넣지 않는다(게시판 설정이 바뀔 때 재색인하지 않도록) — 노출은 조회 시 board_id 필터.
type Doc struct {
	ID           int64  `json:"id"`
	BoardID      int64  `json:"board_id"`
	UserID       int64  `json:"user_id"`
	Title        string `json:"title"`
	CreatedAt    int64  `json:"created_at"`
	UpdatedAt    int64  `json:"updated_at"`
	CommentCount int64  `json:"comment_count"`
}

// IndexSettings — 색인 설정. id 는 대조 시 범위 필터용.
func IndexSettings(maxTotalHits int) *meili.Settings {
	return &meili.Settings{
		FilterableAttributes: []string{"board_id", "user_id", "id"},
		SortableAttributes:   []string{"id"},
		SearchableAttributes: []string{"title"},
		Pagination:           &meili.Pagination{MaxTotalHits: int64(maxTotalHits)},
	}
}

// FromProto — board 원본(IndexPost) → 색인 문서.
func FromProto(p *boardv1.IndexPost) Doc {
	return Doc{
		ID:           p.GetId(),
		BoardID:      p.GetBoardId(),
		UserID:       p.GetUserId(),
		Title:        p.GetTitle(),
		CreatedAt:    p.GetCreatedAtMs(),
		UpdatedAt:    p.GetUpdatedAtMs(),
		CommentCount: p.GetCommentCount(),
	}
}
