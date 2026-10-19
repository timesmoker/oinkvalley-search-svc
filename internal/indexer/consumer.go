package indexer

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/oinkvalley/search/internal/messaging"
)

// 글·댓글 이벤트 공통으로 postId 만 쓴다.
type postRef struct {
	PostID int64 `json:"postId"`
}

// Subjects — 색인에 영향을 주는 board 이벤트. 댓글은 댓글 수 갱신용.
var Subjects = []string{
	"board.post.created",
	"board.post.updated",
	"board.post.deleted",
	"board.comment.created",
}

// Register — 이벤트 구독. 처리 실패는 Nak → JetStream 재전송.
func (x *Indexer) Register(ctx context.Context, bus *messaging.Bus) error {
	for _, subject := range Subjects {
		if err := bus.Subscribe(ctx, subject, x.handle); err != nil {
			return err
		}
	}
	return nil
}

func (x *Indexer) handle(ctx context.Context, _ string, data []byte) error {
	var ref postRef
	if err := json.Unmarshal(data, &ref); err != nil {
		return fmt.Errorf("%w: bad json: %v", messaging.ErrTerm, err)
	}
	if ref.PostID < 1 {
		return fmt.Errorf("%w: missing postId", messaging.ErrTerm)
	}
	return x.Sync(ctx, []int64{ref.PostID})
}
