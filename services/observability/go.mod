module github.com/your-org/helmix/services/observability

go 1.23.0

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/nats-io/nats.go v1.41.0
	github.com/prometheus/client_golang v1.22.0
	github.com/your-org/helmix/libs/event-sdk v0.0.0
)

replace github.com/your-org/helmix/libs/event-sdk => ../../libs/event-sdk
