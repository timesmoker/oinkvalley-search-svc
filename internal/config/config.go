// Package config — 프로세스 설정 (caarlos0/env).
//
// 배포 값 SOT = infra (k3s/values/search). 여기 코드는 환경 변수 "이름"만 고정하고 기본값을 두지 않는다 —
// 빠진 값은 기동 시 한 번에 모아 실패시킨다. API 파드와 batch(CronJob) 파드가 같은 values 로 같은 env 를 받는다.
package config

import (
	"strings"
	"time"

	"github.com/caarlos0/env/v11"
)

// Config 는 기동 후 불변으로 취급한다.
type Config struct {
	Port      string `env:"PORT,notEmpty"`
	JWTSecret string `env:"JWT_SECRET,notEmpty"`

	// NATS — JetStream (색인 이벤트 구독 + listable 게시판 KV).
	NATSURL string `env:"NATS_URL,notEmpty"`
	// NATSDurable — durable base. subject 마다 base-suffix.
	NATSDurable       string        `env:"NATS_DURABLE,notEmpty"`
	NATSReconnectWait time.Duration `env:"NATS_RECONNECT_WAIT,notEmpty"`
	NATSAckWait       time.Duration `env:"NATS_ACK_WAIT,notEmpty"`
	// NATSNakDelay — 처리 실패 메시지 재전송까지 대기
	NATSNakDelay   time.Duration `env:"NATS_NAK_DELAY,notEmpty"`
	NATSMaxDeliver int           `env:"NATS_MAX_DELIVER,notEmpty"`

	MeiliURL         string        `env:"MEILI_URL,notEmpty"`
	MeiliKey         string        `env:"MEILI_MASTER_KEY,notEmpty"`
	MeiliIndex       string        `env:"MEILI_INDEX,notEmpty"`
	MeiliHTTPTimeout time.Duration `env:"MEILI_HTTP_TIMEOUT,notEmpty"`
	// MeiliTaskPoll — 비동기 작업 완료 확인 간격
	MeiliTaskPoll time.Duration `env:"MEILI_TASK_POLL_INTERVAL,notEmpty"`
	// MeiliEnsureRetry — 기동 시 색인 설정 실패 후 재시도 간격 (Meilisearch 가 늦게 떠도 맞춰지게)
	MeiliEnsureRetry time.Duration `env:"MEILI_ENSURE_RETRY_INTERVAL,notEmpty"`

	// MaxTotalHits — 둘러보기 깊은 페이지 상한 (Meilisearch pagination.maxTotalHits).
	MaxTotalHits int `env:"BROWSE_MAX_TOTAL_HITS,notEmpty"`

	// BoardGRPCTarget — app-base 차트가 "static://host:port" 로 주입 (Spring 식). BoardTarget() 으로 읽는다.
	BoardGRPCTarget string `env:"SERVICE_BOARD_GRPC_TARGET,notEmpty"`
	// BoardGRPCTimeout — 색인용 단건 묶음 조회(GetIndexPosts).
	BoardGRPCTimeout time.Duration `env:"BOARD_GRPC_TIMEOUT,notEmpty"`
	// BoardGRPCListTimeout — 배치 순회 한 페이지(ListIndexPosts).
	BoardGRPCListTimeout time.Duration `env:"BOARD_GRPC_LIST_TIMEOUT,notEmpty"`

	// BatchPageSize — 재색인·대조 시 board 에서 한 번에 읽는 글 수.
	BatchPageSize int `env:"BATCH_PAGE_SIZE,notEmpty"`
	// BatchFetchSize — 대조 시 색인에서 한 번에 읽는 문서 수.
	BatchFetchSize int `env:"BATCH_FETCH_SIZE,notEmpty"`
}

// Load 는 환경 변수 → Config. 모든 변수가 필수(notEmpty) — 빠진 것을 모두 모아 에러로 돌려준다.
func Load() (Config, error) {
	return env.ParseAs[Config]()
}

// BoardTarget — "static://board-svc:9090" → Go grpc 가 쓰는 "board-svc:9090".
func (c Config) BoardTarget() string {
	return strings.TrimPrefix(c.BoardGRPCTarget, "static://")
}
