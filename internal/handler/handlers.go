// Package handler — HTTP 핸들러와 응답 형식.
package handler

import (
	"github.com/oinkvalley/search/internal/browse"
	"github.com/oinkvalley/search/internal/meili"
)

// Handlers — HTTP 핸들러 묶음.
type Handlers struct {
	Browse *browse.Service
	Meili  *meili.Client
}

func New(svc *browse.Service, mc *meili.Client) *Handlers {
	return &Handlers{Browse: svc, Meili: mc}
}
