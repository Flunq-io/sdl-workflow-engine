# Timer Service

A minimal event-driven scheduler for wait tasks using Redis ZSET and the shared EventStream.

- Listens to io.flunq.timer.scheduled events
- Stores timers in Redis ZSET (flunq:timers:all)
- Pops due timers via ZPOPMIN and publishes io.flunq.timer.fired
- Uses shared factories to initialize EventStream and Redis

Environment variables:
- REDIS_URL (default: redis:6379)
- REDIS_PASSWORD (optional)
- EVENT_STREAM_TYPE=redis
- TIMER_LOOKAHEAD=100ms
- TIMER_MAX_SLEEP=1s


