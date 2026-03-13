module github.com/your-org/helmix/services/api-gateway

go 1.23.0

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/redis/go-redis/v9 v9.7.0
	github.com/your-org/helmix/libs/auth v0.0.0
	go.opentelemetry.io/otel v1.35.0
	go.opentelemetry.io/otel/trace v1.35.0
)

replace github.com/your-org/helmix/libs/auth => ../../libs/auth
