package handler

import (
	"errors"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"

	"github.com/oinkvalley/search/internal/auth"
	"github.com/oinkvalley/search/internal/browse"
)

// API 계약 상한 (배포마다 바꿀 값이 아니다). 실제로 쓰는 개수는 FE 가 정해 보낸다.
const (
	maxPageSize = 50
	maxByIDs    = 20
)

// BrowsePosts — GET /browse/{key}/posts?page=&size=  (둘 다 필수, page 1부터)
func (h *Handlers) BrowsePosts(w http.ResponseWriter, r *http.Request) {
	page, ok1 := intParam(r, "page")
	size, ok2 := intParam(r, "size")
	if !ok1 || !ok2 || page < 1 || size < 1 || size > maxPageSize {
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("page 는 1 이상, size 는 1~%d 이어야 합니다.", maxPageSize))
		return
	}
	actor := auth.ActorFromContext(r.Context())
	result, err := h.Browse.List(r.Context(), r.PathValue("key"), actor.UserID, actor.Roles, page, size)
	if errors.Is(err, browse.ErrUnknownKey) {
		writeAPIError(w, http.StatusNotFound, "없는 둘러보기입니다.")
		return
	}
	if err != nil {
		// 노출 범위를 모르면 보여주지 않는다 (닫힌 쪽)
		log.Printf("browse: %v", err)
		writeAPIError(w, http.StatusServiceUnavailable, "잠시 후 다시 시도해 주세요.")
		return
	}
	writeJSON(w, http.StatusOK, result)
}

// BrowsePostsByIDs — GET /browse/posts?ids=1,2,3  (최근 본 글 제목 조회, 요청 순서 유지)
func (h *Handlers) BrowsePostsByIDs(w http.ResponseWriter, r *http.Request) {
	raw := strings.TrimSpace(r.URL.Query().Get("ids"))
	var ids []int64
	if raw != "" {
		for _, part := range strings.Split(raw, ",") {
			id, err := strconv.ParseInt(strings.TrimSpace(part), 10, 64)
			if err != nil || id < 1 {
				writeAPIError(w, http.StatusBadRequest, "ids 는 쉼표로 구분한 글 번호여야 합니다.")
				return
			}
			ids = append(ids, id)
		}
	}
	if len(ids) > maxByIDs {
		writeAPIError(w, http.StatusBadRequest, fmt.Sprintf("ids 는 최대 %d개입니다.", maxByIDs))
		return
	}
	actor := auth.ActorFromContext(r.Context())
	posts, err := h.Browse.ByIDs(r.Context(), actor.Roles, ids)
	if err != nil {
		log.Printf("browse by ids: %v", err)
		writeAPIError(w, http.StatusServiceUnavailable, "잠시 후 다시 시도해 주세요.")
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"posts": posts})
}

func intParam(r *http.Request, name string) (int, bool) {
	n, err := strconv.Atoi(r.URL.Query().Get(name))
	return n, err == nil
}
