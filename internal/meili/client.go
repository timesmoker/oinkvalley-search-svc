// Package meili — 공식 SDK(meilisearch-go) 위에 이 서비스가 쓰는 동작만 묶는다.
//
// 쓰기는 비동기 작업(task)이라 여기 쓰기 함수는 모두 작업 성공까지 기다린 뒤 돌아온다.
package meili

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"time"

	ms "github.com/meilisearch/meilisearch-go"
)

// Settings·SearchRequest·DocumentsQuery 는 SDK 타입을 그대로 쓴다.
type (
	Settings       = ms.Settings
	Pagination     = ms.Pagination
	SearchRequest  = ms.SearchRequest
	DocumentsQuery = ms.DocumentsQuery
)

type Client struct {
	sdk      ms.ServiceManager
	taskPoll time.Duration
}

// New — httpTimeout: 요청 하나의 제한, taskPoll: 작업 완료 확인 간격.
func New(url, key string, httpTimeout, taskPoll time.Duration) *Client {
	return &Client{
		sdk:      ms.New(url, ms.WithAPIKey(key), ms.WithCustomClient(&http.Client{Timeout: httpTimeout})),
		taskPoll: taskPoll,
	}
}

// Health — 서버 상태.
func (c *Client) Health(ctx context.Context) error {
	_, err := c.sdk.HealthWithContext(ctx)
	return err
}

// EnsureIndex — 색인이 없으면 만들고(primary key id), 설정을 적용한다.
func (c *Client) EnsureIndex(ctx context.Context, uid string, s *Settings) error {
	_, err := c.sdk.GetIndexWithContext(ctx, uid)
	if isCode(err, "index_not_found") {
		if err := c.wait(ctx)(c.sdk.CreateIndexWithContext(ctx, &ms.IndexConfig{Uid: uid, PrimaryKey: "id"})); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	return c.wait(ctx)(c.sdk.Index(uid).UpdateSettingsWithContext(ctx, s))
}

// DeleteIndex — 색인 삭제. 없는 색인이면 성공으로 본다.
func (c *Client) DeleteIndex(ctx context.Context, uid string) error {
	err := c.wait(ctx)(c.sdk.DeleteIndexWithContext(ctx, uid))
	if isCode(err, "index_not_found") {
		return nil
	}
	return err
}

// Upsert — 문서 추가·갱신.
func (c *Client) Upsert(ctx context.Context, uid string, docs any) error {
	pk := "id"
	return c.wait(ctx)(c.sdk.Index(uid).UpdateDocumentsWithContext(ctx, docs, &ms.DocumentOptions{PrimaryKey: &pk}))
}

// Delete — id 묶음 삭제.
func (c *Client) Delete(ctx context.Context, uid string, ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	keys := make([]string, len(ids))
	for i, id := range ids {
		keys[i] = strconv.FormatInt(id, 10)
	}
	return c.wait(ctx)(c.sdk.Index(uid).DeleteDocumentsWithContext(ctx, keys, nil))
}

// Swap — 두 색인 내용을 맞바꾼다 (무중단 전체 재색인).
func (c *Client) Swap(ctx context.Context, a, b string) error {
	return c.wait(ctx)(c.sdk.SwapIndexesWithContext(ctx, []*ms.SwapIndexesParams{{Indexes: []string{a, b}}}))
}

// SearchResult — 페이지 방식(Page+HitsPerPage) 응답. TotalHits 는 maxTotalHits 까지만 센다.
type SearchResult[T any] struct {
	Hits       []T
	TotalHits  int64
	TotalPages int64
}

// Search — 검색어 없는 필터·정렬 조회도 이걸로 한다.
func Search[T any](ctx context.Context, c *Client, uid string, req *SearchRequest) (SearchResult[T], error) {
	resp, err := c.sdk.Index(uid).SearchWithContext(ctx, req.Query, req)
	if err != nil {
		return SearchResult[T]{}, err
	}
	var hits []T
	if err := resp.Hits.DecodeInto(&hits); err != nil {
		return SearchResult[T]{}, fmt.Errorf("meilisearch decode hits: %w", err)
	}
	return SearchResult[T]{Hits: hits, TotalHits: resp.TotalHits, TotalPages: resp.TotalPages}, nil
}

// FetchResult — 문서 조회 결과 (대조용).
type FetchResult[T any] struct {
	Results []T
	Total   int64
}

func Fetch[T any](ctx context.Context, c *Client, uid string, q *DocumentsQuery) (FetchResult[T], error) {
	var resp ms.DocumentsResult
	if err := c.sdk.Index(uid).GetDocumentsWithContext(ctx, q, &resp); err != nil {
		return FetchResult[T]{}, err
	}
	var out []T
	if err := resp.Results.DecodeInto(&out); err != nil {
		return FetchResult[T]{}, fmt.Errorf("meilisearch decode documents: %w", err)
	}
	return FetchResult[T]{Results: out, Total: resp.Total}, nil
}

// wait — 작업을 넣은 호출 결과를 받아 성공까지 기다린다. 실패한 작업은 코드가 담긴 에러.
func (c *Client) wait(ctx context.Context) func(*ms.TaskInfo, error) error {
	return func(info *ms.TaskInfo, err error) error {
		if err != nil {
			return err
		}
		t, err := c.sdk.WaitForTaskWithContext(ctx, info.TaskUID, c.taskPoll)
		if err != nil {
			return err
		}
		if t.Status != ms.TaskStatusSucceeded {
			return &TaskError{UID: info.TaskUID, Status: string(t.Status), Code: t.Error.Code, Message: t.Error.Message}
		}
		return nil
	}
}

// TaskError — 비동기 작업 실패.
type TaskError struct {
	UID     int64
	Status  string
	Code    string
	Message string
}

func (e *TaskError) Error() string {
	return fmt.Sprintf("meilisearch task %d %s: %s %s", e.UID, e.Status, e.Code, e.Message)
}

// isCode — API 응답 에러나 실패한 작업의 Meilisearch 에러 코드 비교.
func isCode(err error, code string) bool {
	var te *TaskError
	if errors.As(err, &te) {
		return te.Code == code
	}
	var me *ms.Error
	return errors.As(err, &me) && me.MeilisearchApiError.Code == code
}
