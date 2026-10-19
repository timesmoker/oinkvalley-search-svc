package auth

import "context"

// RoleAnonymous — 비로그인 주체의 역할. 서비스 간 계약값: board-svc(BoardActor.ROLE_ANONYMOUS)·
// DB board_role_permissions.role·KV LISTABLE_BOARDS 키와 같아야 한다. 어긋나면 비로그인 둘러보기가 빈 결과(닫힌 쪽).
// 인증 게이트웨이 도입 시 게이트웨이가 역할 헤더로 넘기면 이 상수는 없앤다.
const RoleAnonymous = "ANONYMOUS"

// Actor — 현재 요청 주체. 비로그인은 UserID 0, Roles [ANONYMOUS].
type Actor struct {
	UserID int64
	Roles  []string
}

// Anonymous — 토큰 없는 요청.
func Anonymous() Actor {
	return Actor{Roles: []string{RoleAnonymous}}
}

type contextKey int

const actorKey contextKey = 1

func WithActor(ctx context.Context, a Actor) context.Context {
	return context.WithValue(ctx, actorKey, a)
}

// ActorFromContext — 미들웨어를 안 탔으면 Anonymous.
func ActorFromContext(ctx context.Context) Actor {
	if a, ok := ctx.Value(actorKey).(Actor); ok {
		return a
	}
	return Anonymous()
}
