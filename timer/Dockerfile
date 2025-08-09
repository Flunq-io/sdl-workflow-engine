FROM golang:1.22-alpine AS builder
WORKDIR /app
COPY . /app
WORKDIR /app
RUN --mount=type=cache,target=/go/pkg/mod \
    --mount=type=cache,target=/root/.cache/go-build \
    cd cmd/server && go build -o /timer

FROM alpine:3.19
WORKDIR /srv
COPY --from=builder /timer /srv/timer
ENV REDIS_URL=redis:6379 \
    EVENT_STREAM_TYPE=redis \
    TIMER_LOOKAHEAD=100ms \
    TIMER_MAX_SLEEP=1s
CMD ["/srv/timer"]

