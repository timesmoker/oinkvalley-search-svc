// Package batch — CronJob 이 별도 파드로 실행하는 작업 (API 프로세스와 분리).
//
//	search-svc batch reindex    — 새 색인을 옆에 전부 만든 뒤 교체(무중단), 이어서 대조로 그 사이 변경 반영
//	search-svc batch reconcile  — board(원본) 와 색인을 id 순으로 비교해 누락 추가·잔여 삭제·변경 반영
//
// 원본은 board(Postgres). 어긋나면 항상 원본 기준으로 색인을 고친다.
package batch

import (
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/oinkvalley/search/internal/grpcclient"
	"github.com/oinkvalley/search/internal/meili"
	"github.com/oinkvalley/search/internal/post"
)

// Options — 값은 config(infra values)에서.
type Options struct {
	Index        string
	MaxTotalHits int
	// PageSize — board 에서 한 번에 읽는 글 수
	PageSize int
	// FetchSize — 대조 시 색인에서 한 번에 읽는 문서 수
	FetchSize int
}

type Runner struct {
	grpc  *grpcclient.Clients
	meili *meili.Client
	opt   Options
}

func New(g *grpcclient.Clients, m *meili.Client, opt Options) *Runner {
	return &Runner{grpc: g, meili: m, opt: opt}
}

// Reindex — 전체 재색인.
func (r *Runner) Reindex(ctx context.Context) error {
	settings := post.IndexSettings(r.opt.MaxTotalHits)
	rebuild := r.opt.Index + "_rebuild"
	if err := r.meili.DeleteIndex(ctx, rebuild); err != nil {
		return err
	}
	if err := r.meili.EnsureIndex(ctx, rebuild, settings); err != nil {
		return err
	}
	total := 0
	var after int64
	for {
		page, err := r.grpc.ListIndexPosts(ctx, after, int32(r.opt.PageSize))
		if err != nil {
			return fmt.Errorf("board ListIndexPosts: %w", err)
		}
		docs := make([]post.Doc, 0, len(page.GetPosts()))
		for _, p := range page.GetPosts() {
			docs = append(docs, post.FromProto(p))
		}
		if len(docs) > 0 {
			if err := r.meili.Upsert(ctx, rebuild, docs); err != nil {
				return err
			}
		}
		total += len(docs)
		after = page.GetNextAfterId()
		if !page.GetHasMore() {
			break
		}
	}
	// 교체하려면 양쪽 색인이 다 있어야 한다
	if err := r.meili.EnsureIndex(ctx, r.opt.Index, settings); err != nil {
		return err
	}
	if err := r.meili.Swap(ctx, r.opt.Index, rebuild); err != nil {
		return err
	}
	if err := r.meili.DeleteIndex(ctx, rebuild); err != nil {
		return err
	}
	log.Printf("batch reindex: indexed=%d index=%s", total, r.opt.Index)
	// 재색인 도중 들어온 변경은 옛 색인에 반영됐으므로 원본과 한 번 더 맞춘다
	return r.Reconcile(ctx)
}

type docVersion struct {
	ID           int64 `json:"id"`
	UpdatedAt    int64 `json:"updated_at"`
	CommentCount int64 `json:"comment_count"`
}

// Reconcile — 원본과 색인 대조.
func (r *Runner) Reconcile(ctx context.Context) error {
	if err := r.meili.EnsureIndex(ctx, r.opt.Index, post.IndexSettings(r.opt.MaxTotalHits)); err != nil {
		return err
	}
	var after int64
	upserted, deleted := 0, 0
	for {
		page, err := r.grpc.ListIndexPosts(ctx, after, int32(r.opt.PageSize))
		if err != nil {
			return fmt.Errorf("board ListIndexPosts: %w", err)
		}
		posts := page.GetPosts()
		upper := page.GetNextAfterId()
		if !page.GetHasMore() {
			upper = -1 // 마지막 페이지: 남은 범위 전부 (원본에 없는 큰 id 정리)
		}
		indexed, err := r.indexedRange(ctx, after, upper)
		if err != nil {
			return err
		}
		var docs []post.Doc
		for _, p := range posts {
			v, ok := indexed[p.GetId()]
			if !ok || v.UpdatedAt != p.GetUpdatedAtMs() || v.CommentCount != p.GetCommentCount() {
				docs = append(docs, post.FromProto(p))
			}
			delete(indexed, p.GetId())
		}
		var gone []int64
		for id := range indexed {
			gone = append(gone, id)
		}
		if len(docs) > 0 {
			if err := r.meili.Upsert(ctx, r.opt.Index, docs); err != nil {
				return err
			}
		}
		if err := r.meili.Delete(ctx, r.opt.Index, gone); err != nil {
			return err
		}
		upserted += len(docs)
		deleted += len(gone)
		if !page.GetHasMore() {
			break
		}
		after = upper
	}
	log.Printf("batch reconcile: upserted=%d deleted=%d index=%s", upserted, deleted, r.opt.Index)
	return nil
}

// indexedRange — 색인에서 (after, upper] 범위 문서의 버전. upper<0 이면 상한 없음.
func (r *Runner) indexedRange(ctx context.Context, after, upper int64) (map[int64]docVersion, error) {
	filter := "id > " + strconv.FormatInt(after, 10)
	if upper >= 0 {
		filter += " AND id <= " + strconv.FormatInt(upper, 10)
	}
	out := map[int64]docVersion{}
	var offset int64
	for {
		resp, err := meili.Fetch[docVersion](ctx, r.meili, r.opt.Index, &meili.DocumentsQuery{
			Offset: offset,
			Limit:  int64(r.opt.FetchSize),
			Fields: []string{"id", "updated_at", "comment_count"},
			Filter: filter,
		})
		if err != nil {
			return nil, err
		}
		for _, d := range resp.Results {
			out[d.ID] = d
		}
		offset += int64(len(resp.Results))
		if len(resp.Results) == 0 || offset >= resp.Total {
			return out, nil
		}
	}
}
