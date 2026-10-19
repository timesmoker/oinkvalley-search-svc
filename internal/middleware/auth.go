// Package middleware — HTTP 앞단 공통 처리.
package middleware

import (
	"net/http"
	"strings"

	"github.com/oinkvalley/search/internal/auth"
	"github.com/oinkvalley/search/internal/handler"
)

const bearerPrefix = "Bearer "

// OptionalJWT — 토큰이 없으면 비로그인(ANONYMOUS)으로 통과, 있는데 틀리면 401.
// 둘러보기는 비로그인도 공개 게시판 글을 볼 수 있어야 하므로 필수 인증이 아니다.
func OptionalJWT(v *auth.Validator) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if strings.TrimSpace(header) == "" {
				next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), auth.Anonymous())))
				return
			}
			token, ok := bearerToken(header)
			if !ok {
				handler.WriteInvalidToken(w)
				return
			}
			claims, err := v.Parse(token)
			if err != nil {
				handler.WriteInvalidToken(w)
				return
			}
			actor := auth.Actor{UserID: claims.UserID, Roles: claims.Roles}
			next.ServeHTTP(w, r.WithContext(auth.WithActor(r.Context(), actor)))
		})
	}
}

func bearerToken(header string) (string, bool) {
	if len(header) < len(bearerPrefix) || !strings.EqualFold(header[:len(bearerPrefix)], bearerPrefix) {
		return "", false
	}
	t := strings.TrimSpace(header[len(bearerPrefix):])
	return t, t != ""
}
