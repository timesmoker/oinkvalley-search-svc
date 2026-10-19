// Package listable — 역할별 "목록에 모아 보여도 되는 게시판(listable)" 집합 읽기.
//
// 규칙(활성 + ROLE_READERS + 읽기 권한)의 주인은 board-svc. board 가 계산 결과를 NATS KV(LISTABLE_BOARDS)에 쓰고,
// 여기서는 요청마다 KV 를 읽어 합집합만 만든다 — replica 간 값 차이가 없게 로컬 캐시를 두지 않는다.
// Redis 도입 시 이 패키지의 저장소만 바꾼다.
package listable

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/nats-io/nats.go/jetstream"
)

const Bucket = "LISTABLE_BOARDS"

// Board — listable 게시판 (목록 표시용 slug·이름 포함).
type Board struct {
	ID   int64  `json:"id"`
	Slug string `json:"slug"`
	Name string `json:"name"`
}

type entry struct {
	Version int64   `json:"version"`
	Boards  []Board `json:"boards"`
}

// Store — KV 버킷은 board-svc 가 만든다. 아직 없을 수 있으므로 처음 쓸 때 연다.
type Store struct {
	mu   sync.Mutex
	open func(ctx context.Context) (jetstream.KeyValue, error)
	kv   jetstream.KeyValue
}

func New(open func(ctx context.Context) (jetstream.KeyValue, error)) *Store {
	return &Store{open: open}
}

func (s *Store) bucket(ctx context.Context) (jetstream.KeyValue, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.kv != nil {
		return s.kv, nil
	}
	kv, err := s.open(ctx)
	if err != nil {
		return nil, fmt.Errorf("listable boards kv open: %w", err)
	}
	s.kv = kv
	return kv, nil
}

// Boards — 역할들의 listable 게시판 합집합 (id 오름차순).
// 키가 없는 역할은 "볼 수 있는 게 없음"으로 본다. KV 장애는 에러 — 호출자는 닫힌 쪽(빈 결과/503)으로.
func (s *Store) Boards(ctx context.Context, roles []string) ([]Board, error) {
	kv, err := s.bucket(ctx)
	if err != nil {
		return nil, err
	}
	byID := map[int64]Board{}
	for _, role := range roles {
		e, err := kv.Get(ctx, role)
		if errors.Is(err, jetstream.ErrKeyNotFound) {
			continue
		}
		if err != nil {
			return nil, fmt.Errorf("listable boards kv get %s: %w", role, err)
		}
		var v entry
		if err := json.Unmarshal(e.Value(), &v); err != nil {
			return nil, fmt.Errorf("listable boards kv decode %s: %w", role, err)
		}
		for _, b := range v.Boards {
			byID[b.ID] = b
		}
	}
	out := make([]Board, 0, len(byID))
	for _, b := range byID {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out, nil
}
