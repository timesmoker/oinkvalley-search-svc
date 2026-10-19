# Go 1.22 — go.mod 와 맞춤 (notification-svc 와 같은 구성).
# board gRPC stub 은 비공개 Go 모듈(github.com/timesmoker/oinkvalley-protos/go/board) — 토큰은 secret 으로 그 RUN 안에서만 쓴다.
#   docker build --build-arg GITHUB_ACTOR=<id> --secret id=github_token,env=GITHUB_TOKEN ...
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates git
ARG GITHUB_ACTOR
ENV GOPRIVATE=github.com/timesmoker/*
COPY go.mod go.sum ./
# git 인증은 환경 변수로만 (이미지·레이어에 .gitconfig 를 남기지 않음)
RUN --mount=type=secret,id=github_token \
    GIT_CONFIG_COUNT=1 \
    GIT_CONFIG_KEY_0="url.https://${GITHUB_ACTOR}:$(cat /run/secrets/github_token 2>/dev/null)@github.com/.insteadOf" \
    GIT_CONFIG_VALUE_0="https://github.com/" \
    go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o /server ./cmd/server

FROM alpine:3.21
RUN apk add --no-cache ca-certificates tzdata \
    && adduser -D -H -u 10001 app
USER 10001
WORKDIR /app
COPY --from=build /server /app/server
EXPOSE 8080
ENTRYPOINT ["/app/server"]
