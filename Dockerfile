# Go 1.22 — go.mod 와 맞춤 (notification-svc 와 같은 구성).
# board gRPC stub 은 공개 Go 모듈(github.com/timesmoker/oinkvalley-protos/go/board) — 인증 없이 받는다.
FROM golang:1.22-alpine AS build
WORKDIR /src
RUN apk add --no-cache ca-certificates
COPY go.mod go.sum ./
RUN go mod download
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
