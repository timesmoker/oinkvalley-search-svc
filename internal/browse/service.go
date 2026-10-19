package browse

import (
	"context"
	"strconv"
	"strings"
	"time"

	"github.com/oinkvalley/search/internal/listable"
	"github.com/oinkvalley/search/internal/meili"
	"github.com/oinkvalley/search/internal/post"
)

type Service struct {
	boards       *listable.Store
	meili        *meili.Client
	index        string
	maxTotalHits int
}

func New(boards *listable.Store, m *meili.Client, index string, maxTotalHits int) *Service {
	return &Service{boards: boards, meili: m, index: index, maxTotalHits: maxTotalHits}
}

// List — page 는 1부터.
func (s *Service) List(ctx context.Context, key string, userID int64, roles []string, page, size int) (Page, error) {
	def, ok := Definitions[key]
	if !ok {
		return Page{}, ErrUnknownKey
	}
	empty := Page{Posts: []Post{}, Page: page, TotalPages: 0}

	boards, err := s.boards.Boards(ctx, roles)
	if err != nil {
		return Page{}, err
	}
	if len(boards) == 0 {
		return empty, nil
	}
	byID := make(map[int64]listable.Board, len(boards))
	ids := make([]string, 0, len(boards))
	for _, b := range boards {
		byID[b.ID] = b
		ids = append(ids, strconv.FormatInt(b.ID, 10))
	}
	filter := "board_id IN [" + strings.Join(ids, ", ") + "]"
	if def.Filter != nil {
		if extra := def.Filter(userID); extra != "" {
			filter += " AND (" + extra + ")"
		}
	}

	hitsPerPage := int64(size)
	resp, err := meili.Search[post.Doc](ctx, s.meili, s.index, &meili.SearchRequest{
		Filter:      filter,
		Sort:        []string{"id:desc"},
		Page:        int64(page),
		HitsPerPage: &hitsPerPage,
	})
	if err != nil {
		return Page{}, err
	}

	posts := make([]Post, 0, len(resp.Hits))
	for _, h := range resp.Hits {
		b, ok := byID[h.BoardID]
		if !ok {
			continue // 필터를 통과했으면 있어야 함 — 방어
		}
		posts = append(posts, Post{
			ID:           h.ID,
			BoardSlug:    b.Slug,
			BoardName:    b.Name,
			Title:        h.Title,
			CommentCount: h.CommentCount,
			AuthorUserID: h.UserID,
			CreatedAt:    time.UnixMilli(h.CreatedAt).UTC().Format(time.RFC3339),
		})
	}
	return Page{
		Posts:      posts,
		Page:       page,
		TotalPages: int(resp.TotalPages),
		Capped:     resp.TotalHits >= int64(s.maxTotalHits),
	}, nil
}

// ByIDs — 글 id 묶음을 요청 순서대로. listable 게시판 글만 (못 보는·삭제된 글은 빠진다).
// FE 최근 본 글 목록이 id 만 저장해 두었다가 제목을 받을 때 쓴다.
func (s *Service) ByIDs(ctx context.Context, roles []string, postIDs []int64) ([]Post, error) {
	if len(postIDs) == 0 {
		return []Post{}, nil
	}
	boards, err := s.boards.Boards(ctx, roles)
	if err != nil {
		return nil, err
	}
	if len(boards) == 0 {
		return []Post{}, nil
	}
	byID := make(map[int64]listable.Board, len(boards))
	boardIDs := make([]string, 0, len(boards))
	for _, b := range boards {
		byID[b.ID] = b
		boardIDs = append(boardIDs, strconv.FormatInt(b.ID, 10))
	}
	ids := make([]string, 0, len(postIDs))
	for _, id := range postIDs {
		ids = append(ids, strconv.FormatInt(id, 10))
	}
	hitsPerPage := int64(len(postIDs))
	resp, err := meili.Search[post.Doc](ctx, s.meili, s.index, &meili.SearchRequest{
		Filter:      "id IN [" + strings.Join(ids, ", ") + "] AND board_id IN [" + strings.Join(boardIDs, ", ") + "]",
		Page:        1,
		HitsPerPage: &hitsPerPage,
	})
	if err != nil {
		return nil, err
	}
	found := make(map[int64]Post, len(resp.Hits))
	for _, h := range resp.Hits {
		b, ok := byID[h.BoardID]
		if !ok {
			continue
		}
		found[h.ID] = Post{
			ID:           h.ID,
			BoardSlug:    b.Slug,
			BoardName:    b.Name,
			Title:        h.Title,
			CommentCount: h.CommentCount,
			AuthorUserID: h.UserID,
			CreatedAt:    time.UnixMilli(h.CreatedAt).UTC().Format(time.RFC3339),
		}
	}
	out := make([]Post, 0, len(found))
	for _, id := range postIDs {
		if p, ok := found[id]; ok {
			out = append(out, p)
		}
	}
	return out, nil
}
