// Package browse — 둘러보기(전체 게시글 등) 조회.
//
// 고르기는 Meilisearch, 노출 범위는 listable 게시판 집합(board 가 KV 에 쓴 결과)으로 거른다.
// 글 내용(제목 등)은 색인 문서에서, 게시판 이름은 같은 집합에서 붙인다.
// 작성자 닉네임은 다른 목록과 같이 FE 가 profile 에서 묶어 붙인다(여기서는 authorUserId 만).
// 표시(주소·이름 대체 문구)는 FE 가 정한다. 글 상세 권한은 board 가 다시 검사한다.
package browse

import "errors"

var ErrUnknownKey = errors.New("unknown browse key")

// Definition — 둘러보기 종류. 기본 노출 필터(listable 게시판)는 List 가 항상 붙이고, 정의는 추가 조건만 갖는다.
type Definition struct {
	Filter func(actorUserID int64) string // 추가 필터 (없으면 nil)
}

// Definitions — FE 의 BROWSE_DEFINITIONS 와 키를 맞춘다.
var Definitions = map[string]Definition{
	"all": {},
}

type Post struct {
	ID           int64  `json:"id"`
	BoardSlug    string `json:"boardSlug"`
	BoardName    string `json:"boardName"`
	Title        string `json:"title"`
	CommentCount int64  `json:"commentCount"`
	AuthorUserID int64  `json:"authorUserId"`
	AIGenerated  bool   `json:"aiGenerated"`
	CreatedAt    string `json:"createdAt"`
}

// Page — 페이지 번호 방식. Capped 면 전체 개수가 상한(maxTotalHits)에 걸린 것 — TotalPages 너머에도 글이 있다.
type Page struct {
	Posts      []Post `json:"posts"`
	Page       int    `json:"page"`
	TotalPages int    `json:"totalPages"`
	Capped     bool   `json:"capped"`
}
