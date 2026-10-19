// Package grpcclient — board 내부 gRPC 클라이언트 (클러스터 내부 전용, 평문). 색인 원천 조회용.
package grpcclient

import (
	"context"
	"fmt"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/timesmoker/oinkvalley-protos/go/board/boardv1"
)

// Options — 값은 config(infra values)에서.
type Options struct {
	BoardTarget string
	// CallTimeout — 색인용 단건 묶음 조회(GetIndexPosts)
	CallTimeout time.Duration
	// ListTimeout — 배치 순회 한 페이지(ListIndexPosts)
	ListTimeout time.Duration
}

type Clients struct {
	conn  *grpc.ClientConn
	opt   Options
	Board boardv1.BoardInternalServiceClient
}

func Dial(opt Options) (*Clients, error) {
	bc, err := grpc.NewClient(opt.BoardTarget, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("board grpc: %w", err)
	}
	return &Clients{conn: bc, opt: opt, Board: boardv1.NewBoardInternalServiceClient(bc)}, nil
}

func (c *Clients) Close() {
	_ = c.conn.Close()
}

// IndexPosts — 글 id 묶음의 현재 상태. 응답에 없는 id = 삭제된 글.
func (c *Clients) IndexPosts(ctx context.Context, ids []int64) ([]*boardv1.IndexPost, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opt.CallTimeout)
	defer cancel()
	resp, err := c.Board.GetIndexPosts(ctx, &boardv1.GetIndexPostsRequest{PostIds: ids})
	if err != nil {
		return nil, err
	}
	return resp.GetPosts(), nil
}

// ListIndexPosts — id 오름차순 순회 한 페이지.
func (c *Clients) ListIndexPosts(ctx context.Context, afterID int64, limit int32) (*boardv1.ListIndexPostsResponse, error) {
	ctx, cancel := context.WithTimeout(ctx, c.opt.ListTimeout)
	defer cancel()
	return c.Board.ListIndexPosts(ctx, &boardv1.ListIndexPostsRequest{AfterId: afterID, Limit: limit})
}
