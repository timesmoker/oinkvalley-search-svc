// Package auth — JWT 검증 + request 에 주체(user id, roles) 넣기.
//
// notification/stamp-rally 와 같은 방식: auth-svc 가 발급한 HS256 JWT 를 직접 파싱 (외부 라이브러리 없음).
// 차이: 역할(roles 배열)로 볼 수 있는 게시판을 고르므로 payload 를 정규식이 아니라 encoding/json 으로 파싱한다.
//
// Traefik 역할(참고): 쿠키 → Authorization: Bearer 복사만. 서명·만료 검증은 이 패키지가 한다.
// 추후 인증 게이트웨이가 생기면 게이트웨이가 넘기는 헤더를 읽도록 바꾼다.
package auth

import (
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
)

var (
	ErrMalformedJWT = errors.New("malformed jwt")
	ErrBadSignature = errors.New("invalid jwt signature")
	ErrMissingSub   = errors.New("jwt missing numeric sub")
	ErrExpired      = errors.New("jwt expired")
)

// Claims — 핸들러가 쓰는 최소 정보. Roles 는 ROLE_ 접두사 제거 (board 와 동일).
type Claims struct {
	UserID int64
	Roles  []string
}

// Validator 는 JWT_SECRET 을 들고 토큰을 검증한다.
type Validator struct {
	secret []byte
}

// NewValidator — secret 은 auth / 다른 백엔드와 동일한 JWT_SECRET. 32바이트 미만 거절.
func NewValidator(secret string) (*Validator, error) {
	b := []byte(secret)
	if len(b) < 32 {
		return nil, fmt.Errorf("JWT_SECRET must be at least 32 bytes (UTF-8)")
	}
	return &Validator{secret: b}, nil
}

// Parse — "header.payload.signature" 검증 후 Claims 반환.
func (v *Validator) Parse(token string) (Claims, error) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return Claims{}, ErrMalformedJWT
	}
	expected := signHS256(v.secret, parts[0]+"."+parts[1])
	actual, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return Claims{}, ErrMalformedJWT
	}
	if subtle.ConstantTimeCompare(expected, actual) != 1 {
		return Claims{}, ErrBadSignature
	}
	payload, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return Claims{}, ErrMalformedJWT
	}
	return parsePayload(payload)
}

type payload struct {
	Sub   json.RawMessage `json:"sub"`
	Exp   *int64          `json:"exp"`
	Roles []string        `json:"roles"`
}

func parsePayload(raw []byte) (Claims, error) {
	var p payload
	if err := json.Unmarshal(raw, &p); err != nil {
		return Claims{}, ErrMalformedJWT
	}
	if p.Exp != nil && time.Now().Unix() > *p.Exp {
		return Claims{}, ErrExpired
	}
	// sub 는 "12" 또는 12 둘 다 허용
	sub := strings.Trim(string(p.Sub), `"`)
	userID, err := strconv.ParseInt(sub, 10, 64)
	if err != nil || userID < 1 {
		return Claims{}, ErrMissingSub
	}
	roles := make([]string, 0, len(p.Roles))
	for _, r := range p.Roles {
		r = strings.TrimPrefix(strings.TrimSpace(r), "ROLE_")
		if r != "" {
			roles = append(roles, r)
		}
	}
	return Claims{UserID: userID, Roles: roles}, nil
}

func signHS256(secret []byte, data string) []byte {
	mac := hmac.New(sha256.New, secret)
	_, _ = mac.Write([]byte(data))
	return mac.Sum(nil)
}
