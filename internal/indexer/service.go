// Package indexer — board 이벤트 → Meilisearch 문서.
//
// 이벤트에는 postId 만 있다(얇은 이벤트). 항상 board 에서 현재 상태를 다시 읽어 덮어쓰므로
// 순서가 뒤바뀌거나 중복 수신돼도 결과가 최신이다. board 응답에 없는 id = 삭제된 글 → 색인에서 삭제.
package indexer

import (
	"context"
	"fmt"

	"github.com/oinkvalley/search/internal/grpcclient"
	"github.com/oinkvalley/search/internal/meili"
	"github.com/oinkvalley/search/internal/post"
)

type Indexer struct {
	grpc  *grpcclient.Clients
	meili *meili.Client
	index string
}

func New(grpc *grpcclient.Clients, m *meili.Client, index string) *Indexer {
	return &Indexer{grpc: grpc, meili: m, index: index}
}

// Sync — id 들의 현재 상태로 색인을 맞춘다 (반영 완료까지 대기).
func (x *Indexer) Sync(ctx context.Context, ids []int64) error {
	posts, err := x.grpc.IndexPosts(ctx, ids)
	if err != nil {
		return fmt.Errorf("board GetIndexPosts: %w", err)
	}
	present := make(map[int64]bool, len(posts))
	docs := make([]post.Doc, 0, len(posts))
	for _, p := range posts {
		present[p.GetId()] = true
		docs = append(docs, post.FromProto(p))
	}
	var gone []int64
	for _, id := range ids {
		if !present[id] {
			gone = append(gone, id)
		}
	}
	if len(docs) > 0 {
		if err := x.meili.Upsert(ctx, x.index, docs); err != nil {
			return err
		}
	}
	return x.meili.Delete(ctx, x.index, gone)
}
