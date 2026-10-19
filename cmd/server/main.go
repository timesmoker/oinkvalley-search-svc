// Package main — search-svc 진입점.
//
//	search-svc                  — HTTP(둘러보기 API) + JetStream 구독(색인)
//	search-svc batch reindex    — 전체 재색인 (CronJob/수동)
//	search-svc batch reconcile  — 원본 대조 (야간 CronJob)
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/nats-io/nats.go/jetstream"

	"github.com/oinkvalley/search/internal/auth"
	"github.com/oinkvalley/search/internal/batch"
	"github.com/oinkvalley/search/internal/browse"
	"github.com/oinkvalley/search/internal/config"
	"github.com/oinkvalley/search/internal/grpcclient"
	"github.com/oinkvalley/search/internal/handler"
	"github.com/oinkvalley/search/internal/indexer"
	"github.com/oinkvalley/search/internal/listable"
	"github.com/oinkvalley/search/internal/meili"
	"github.com/oinkvalley/search/internal/messaging"
	"github.com/oinkvalley/search/internal/middleware"
	"github.com/oinkvalley/search/internal/post"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatal(err)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	clients, err := grpcclient.Dial(grpcclient.Options{
		BoardTarget: cfg.BoardTarget(),
		CallTimeout: cfg.BoardGRPCTimeout,
		ListTimeout: cfg.BoardGRPCListTimeout,
	})
	if err != nil {
		log.Fatalf("grpc: %v", err)
	}
	defer clients.Close()
	mc := meili.New(cfg.MeiliURL, cfg.MeiliKey, cfg.MeiliHTTPTimeout, cfg.MeiliTaskPoll)

	if len(os.Args) > 1 && os.Args[1] == "batch" {
		runBatch(ctx, cfg, clients, mc, os.Args[2:])
		return
	}
	serve(ctx, cfg, clients, mc)
}

func runBatch(ctx context.Context, cfg config.Config, clients *grpcclient.Clients, mc *meili.Client, args []string) {
	runner := batch.New(clients, mc, batch.Options{
		Index:        cfg.MeiliIndex,
		MaxTotalHits: cfg.MaxTotalHits,
		PageSize:     cfg.BatchPageSize,
		FetchSize:    cfg.BatchFetchSize,
	})
	if len(args) != 1 {
		log.Fatal("usage: search-svc batch reindex|reconcile")
	}
	var err error
	switch args[0] {
	case "reindex":
		err = runner.Reindex(ctx)
	case "reconcile":
		err = runner.Reconcile(ctx)
	default:
		log.Fatalf("unknown batch command %q", args[0])
	}
	if err != nil {
		log.Fatalf("batch %s: %v", args[0], err)
	}
}

func serve(ctx context.Context, cfg config.Config, clients *grpcclient.Clients, mc *meili.Client) {
	validator, err := auth.NewValidator(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("jwt: %v", err)
	}

	bus, err := messaging.Connect(messaging.Options{
		URL:           cfg.NATSURL,
		DurableBase:   cfg.NATSDurable,
		ReconnectWait: cfg.NATSReconnectWait,
		AckWait:       cfg.NATSAckWait,
		NakDelay:      cfg.NATSNakDelay,
		MaxDeliver:    cfg.NATSMaxDeliver,
	})
	if err != nil {
		log.Fatalf("nats: %v", err)
	}
	defer bus.Close()

	// 색인 설정은 Meilisearch 가 늦게 떠도 맞춰지도록 재시도
	go func() {
		for {
			err := mc.EnsureIndex(ctx, cfg.MeiliIndex, post.IndexSettings(cfg.MaxTotalHits))
			if err == nil {
				log.Printf("meilisearch index ready: %s", cfg.MeiliIndex)
				break
			}
			log.Printf("meilisearch ensure index: %v (retry)", err)
			select {
			case <-ctx.Done():
				return
			case <-time.After(cfg.MeiliEnsureRetry):
			}
		}
		if err := indexer.New(clients, mc, cfg.MeiliIndex).Register(ctx, bus); err != nil {
			log.Printf("nats indexer: %v", err)
		}
	}()

	boards := listable.New(func(ctx context.Context) (jetstream.KeyValue, error) { return bus.KeyValue(ctx, listable.Bucket) })
	h := handler.New(browse.New(boards, mc, cfg.MeiliIndex, cfg.MaxTotalHits), mc)
	optionalJWT := middleware.OptionalJWT(validator)

	mux := http.NewServeMux()
	mux.HandleFunc("GET /health", h.Health)
	mux.Handle("GET /browse/{key}/posts", optionalJWT(http.HandlerFunc(h.BrowsePosts)))
	mux.Handle("GET /browse/posts", optionalJWT(http.HandlerFunc(h.BrowsePostsByIDs)))

	srv := &http.Server{Addr: ":" + cfg.Port, Handler: mux}
	go func() {
		<-ctx.Done()
		_ = srv.Shutdown(context.Background())
	}()

	log.Printf("search-svc listening on %s", srv.Addr)
	if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatal(err)
	}
}
