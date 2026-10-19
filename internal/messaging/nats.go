// Package messaging — NATS JetStream 연결·구독·KV (nats.go jetstream 패키지). notification-svc 의 Bus 와 같은 형태.
//
// 구독은 durable pull consumer 를 명시적으로 만들어 붙는다. 파드가 내려가도 consumer 는 서버에 남고,
// replica 들은 같은 consumer 에서 나눠 받는다.
package messaging

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// ErrTerm — 다시 보내도 소용없는 메시지 (형식 오류 등). Term 처리.
var ErrTerm = errors.New("terminal message")

// Handler — 처리 성공 시 nil → Ack. 그 외 Nak(재전송), ErrTerm 이면 Term.
type Handler func(ctx context.Context, subject string, data []byte) error

// Options — 값은 config(infra values)에서.
type Options struct {
	URL           string
	DurableBase   string
	ReconnectWait time.Duration
	AckWait       time.Duration
	NakDelay      time.Duration
	MaxDeliver    int
}

// Bus — JetStream 구독 묶음.
type Bus struct {
	nc   *nats.Conn
	js   jetstream.JetStream
	opt  Options
	subs []jetstream.ConsumeContext
}

func Connect(opt Options) (*Bus, error) {
	nc, err := nats.Connect(opt.URL,
		nats.Name("search-svc"),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(opt.ReconnectWait),
	)
	if err != nil {
		return nil, fmt.Errorf("nats connect: %w", err)
	}
	js, err := jetstream.New(nc)
	if err != nil {
		nc.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}
	return &Bus{nc: nc, js: js, opt: opt}, nil
}

// KeyValue — 기존 버킷 열기 (버킷은 board-svc 가 만든다).
func (b *Bus) KeyValue(ctx context.Context, bucket string) (jetstream.KeyValue, error) {
	return b.js.KeyValue(ctx, bucket)
}

// Subscribe — subject 하나를 durable(base + "-" + suffix) consumer 로 받는다.
func (b *Bus) Subscribe(ctx context.Context, subject string, h Handler) error {
	durable := b.opt.DurableBase + "-" + durableSuffix(subject)
	stream, err := b.js.StreamNameBySubject(ctx, subject)
	if err != nil {
		return fmt.Errorf("stream for %s (run nats-ensure-subjects.sh?): %w", subject, err)
	}
	cons, err := ensureConsumer(ctx, b.nc, b.js, stream, jetstream.ConsumerConfig{
		Durable:       durable,
		FilterSubject: subject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		AckWait:       b.opt.AckWait,
		MaxDeliver:    b.opt.MaxDeliver,
		// 처음 만들 때 스트림에 남은 과거 이벤트는 건너뜀 — 기존 글은 전체 재색인이 채운다
		DeliverPolicy: jetstream.DeliverNewPolicy,
	})
	if err != nil {
		return fmt.Errorf("consumer %s: %w", durable, err)
	}
	cc, err := cons.Consume(func(msg jetstream.Msg) {
		err := h(context.Background(), msg.Subject(), msg.Data())
		if err == nil {
			_ = msg.Ack()
			return
		}
		if errors.Is(err, ErrTerm) {
			log.Printf("nats: term subject=%s: %v", msg.Subject(), err)
			_ = msg.Term()
			return
		}
		log.Printf("nats: nak subject=%s: %v", msg.Subject(), err)
		_ = msg.NakWithDelay(b.opt.NakDelay)
	})
	if err != nil {
		return fmt.Errorf("consume %s: %w", durable, err)
	}
	b.subs = append(b.subs, cc)
	log.Printf("nats: subscribed subject=%s stream=%s durable=%s", subject, stream, durable)
	return nil
}

func (b *Bus) Connected() bool {
	return b.nc != nil && b.nc.IsConnected()
}

// Close — 받던 메시지를 마저 처리하고 연결을 닫는다. consumer 는 서버에 남는다.
func (b *Bus) Close() {
	for _, cc := range b.subs {
		cc.Drain()
	}
	if b.nc != nil {
		b.nc.Close()
	}
}

// ensureConsumer — durable pull consumer 를 만들거나 설정(ack 대기·재전송 상한)을 맞춘다.
// 시작 위치(deliver policy)는 처음 만들 때만 정해지고 이후엔 바뀌지 않는다.
// 예전 QueueSubscribe 가 만든 push consumer 가 남아 있으면 ack 된 위치 다음부터 이어 받도록 pull 로 바꾼다.
func ensureConsumer(ctx context.Context, nc *nats.Conn, js jetstream.JetStream, stream string, want jetstream.ConsumerConfig) (jetstream.Consumer, error) {
	legacy, err := nc.JetStream()
	if err != nil {
		return nil, err
	}
	info, err := legacy.ConsumerInfo(stream, want.Durable, nats.Context(ctx))
	switch {
	case errors.Is(err, nats.ErrConsumerNotFound):
		return js.CreateConsumer(ctx, stream, want)
	case err != nil:
		return nil, err
	case info.Config.DeliverSubject != "":
		log.Printf("nats: migrating push consumer %s to pull from stream seq %d", want.Durable, info.AckFloor.Stream+1)
		if err := js.DeleteConsumer(ctx, stream, want.Durable); err != nil && !errors.Is(err, jetstream.ErrConsumerNotFound) {
			return nil, err
		}
		want.DeliverPolicy = jetstream.DeliverByStartSequencePolicy
		want.OptStartSeq = info.AckFloor.Stream + 1
		return js.CreateConsumer(ctx, stream, want)
	default:
		cons, err := js.Consumer(ctx, stream, want.Durable)
		if err != nil {
			return nil, err
		}
		cfg := cons.CachedInfo().Config
		cfg.AckWait, cfg.MaxDeliver, cfg.FilterSubject = want.AckWait, want.MaxDeliver, want.FilterSubject
		return js.UpdateConsumer(ctx, stream, cfg)
	}
}

func durableSuffix(subject string) string {
	s := strings.ReplaceAll(subject, ".", "-")
	s = strings.ReplaceAll(s, ">", "all")
	s = strings.ReplaceAll(s, "*", "x")
	return s
}
