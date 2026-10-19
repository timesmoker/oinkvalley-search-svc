# search-svc (Go)

전체 게시글(둘러보기)·검색 읽기 모델. 원본은 board(Postgres) — 여기는 Meilisearch 색인을 채우고 읽기만 한다.
board 쪽(이벤트·listable KV·색인 RPC)은 board README 의 "이벤트·검색 연동".

배포·env SOT는 **`infra`** (`k3s/values/search`). 코드에 기본값 없음 — 빠진 env 는 기동 시 모아서 실패.

## HTTP API (JWT 선택 — 없으면 비로그인 ANONYMOUS)

| 메서드 | 경로 | 설명 |
|--------|------|------|
| `GET` | `/health` | Meilisearch 상태 (인증 없음) |
| `GET` | `/browse/{key}/posts?page=&size=` | 둘러보기 한 페이지. 둘 다 필수, page 1부터, size 1~50. `{posts, page, totalPages, capped}` |
| `GET` | `/browse/posts?ids=1,2,3` | 글 id 묶음 (최대 20, 요청 순서). 볼 수 있는 글만 — FE 최근 본 글 제목 갱신용 |

`key` 는 `internal/browse/model.go` 의 `Definitions` (FE `BROWSE_DEFINITIONS` 와 키를 맞춘다). 지금은 `all`.

노출 범위: board 가 NATS KV `LISTABLE_BOARDS` 에 역할별로 쓴 게시판 집합 (활성 + 글 읽기 정책 ROLE_READERS + 그 역할에 읽기 권한).

## 색인

| | 내용 |
|--|------|
| 구독 | `board.post.created` / `updated` / `deleted`, `board.comment.created` (durable pull consumer `NATS_DURABLE-<subject>`) |
| 처리 | 이벤트의 `postId` 로 board gRPC `GetIndexPosts` 를 다시 읽어 덮어쓰기 (없으면 삭제) |
| 배치 | `search-svc batch reconcile` — 매일 03:30 CronJob `search-svc-reconcile` (원본과 대조) |
| | `search-svc batch reindex` — 수동 전체 재색인 (`infra/k3s/cronjobs/README.md`) |

## 설계

### 원칙

- **Postgres(board)가 SSOT**, Meilisearch 는 파생 사본. 어긋나면 원본 기준으로 고치고, 사본은 백업하지 않는다(재색인으로 복구).
- Meilisearch 에 쓰고 읽는 건 **search-svc 하나뿐** — board 가 Postgres·Meilisearch 에 각각 쓰는 이중 쓰기 금지. 권한 필터가 질의에 들어가므로 브라우저가 직접 붙으면 안 된다.
- 권한 규칙은 재구현하지 않는다 — board 가 계산한 listable 집합만 쓴다. 글 상세의 최종 권한 검사는 board.
- JWT 는 여기서 직접 검증한다 (Traefik 은 쿠키를 헤더로 옮길 뿐). 인증 게이트웨이가 생기면 그 헤더만 읽게 바꾼다.

### 조회

| 항목 | 값 |
|---|---|
| 필터 | `board_id IN [<역할별 listable 합집합>]` (+ 둘러보기 정의의 추가 조건) |
| 정렬 | `id:desc` — 단일 시퀀스라 id 순서 = 등록 순서. 과거 데이터 이관·예약 발행·끌어올리기·DB 분할을 도입하면 재검토 |
| 페이지 | 페이지 번호(`page`/`hitsPerPage`). 전체 개수는 `BROWSE_MAX_TOTAL_HITS` 까지만 세고, 걸리면 `capped` → FE 가 마지막 번호에 "+" |

볼 수 있는 게시판이 없으면(예: `GUEST`) 질의 없이 빈 결과. 색인 반영까지 짧은 지연이 있어 방금 쓴 글이 바로 안 보일 수 있다.

### 색인 문서 (`posts`)

`id`(sortable, filterable — 대조 범위용), `board_id`(filterable), `user_id`(filterable), `title`(searchable), `created_at`, `updated_at`, `comment_count`.

- 노출 플래그·역할·`is_active` 같은 **가시성 필드는 넣지 않는다** — 넣으면 게시판 설정·권한이 바뀔 때마다 그 게시판 전체 재색인.
- 비활성 게시판 글도 지우지 않는다 — 조회 필터로 빠지고(비용 0), 재활성화 때 다시 넣을 일도 없다.
- 닉네임은 다른 목록과 같이 FE 가 profile 에서 묶어 붙인다 (여기서는 `authorUserId` 만).

### 배치

API 프로세스 안에서 돌리지 않고, 같은 이미지의 `batch` 명령을 CronJob 이 별도 파드로 실행한다 (replica 수만큼 중복 실행·API 자원 잠식 방지, `concurrencyPolicy: Forbid`). 재색인은 새 색인을 옆에 전부 만든 뒤 교체(swap) — 만드는 동안 기존 색인으로 서비스.

### 채택하지 않은 안

| 안 | 이유 |
|---|---|
| Postgres 로 여러 게시판 글 최신순 (PK 순 훑기 / 게시판별 LATERAL 병합 / 역할별 피드 테이블) | 볼 수 있는 글이 드물면 전체 스캔, 게시판 수 × 깊이 비용, 권한 변경마다 행 재정리(어긋나면 권한 누출) |
| 역할 조합별 그룹, 글에 공개 등급 | 조합 폭발, 역할에 상하 관계가 없음 |
| 피드에 권한 없는 게시판 글 노출 후 상세에서만 차단 | 필터 제외 비용이 0인데 제목 노출·개수 부풀림 |
| Elasticsearch | 운영(8GB Raspberry Pi)에 힙 1GB+ 부담. 개념은 그대로 이전 가능 |
| Typesense | 전부 메모리 상주, 한국어 처리 약함 |
| Qdrant 로 의미 검색 | 키워드와 별도 색인·동기화·결과 병합 필요 — 의미 검색이 필요해지면 벡터만 분리 검토 |

## 구조

```
search/
  cmd/server/main.go       # HTTP + 구독, batch 서브커맨드
  internal/config/         # env → Config (caarlos0/env, 전부 필수)
  internal/auth/           # JWT 검증, Actor
  internal/middleware/     # OptionalJWT
  internal/handler/        # HTTP 핸들러·응답 형식
  internal/browse/         # 둘러보기 조회 (model + service)
  internal/indexer/        # 이벤트 → 색인 (service + consumer)
  internal/batch/          # reindex / reconcile
  internal/post/           # 색인 문서·설정 (indexer·browse·batch 공용)
  internal/listable/       # KV LISTABLE_BOARDS 읽기
  internal/meili/          # Meilisearch SDK 래퍼
  internal/messaging/      # NATS JetStream (pull consumer)
  internal/grpcclient/     # board 내부 gRPC
```

## 이미지 배포

태그 push 시 GitHub Actions(`.github/workflows/publish.yml`)가 `ghcr.io/<owner>/search-svc` arm64 이미지를 올린다.
board gRPC stub 은 비공개 `oinkvalley-protos` 의 Go 모듈이라, 저장소 secret **`PROTOS_READ_TOKEN`**(oinkvalley-protos Contents read 만 준 fine-grained token)이 필요하다.

## 로컬

```bash
./infra/k3s/scripts/k3s-helm-upgrade-service.sh search   # env 바뀌었을 때 먼저
./infra/k3s/scripts/k3s-reload-service.sh search
```
