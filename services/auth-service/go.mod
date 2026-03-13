module github.com/your-org/helmix/services/auth-service

go 1.23.0

require (
	github.com/go-chi/chi/v5 v5.1.0
	github.com/jackc/pgx/v5 v5.7.2
	github.com/redis/go-redis/v9 v9.7.0
	github.com/your-org/helmix/libs/auth v0.0.0
)

replace github.com/your-org/helmix/libs/auth => ../../libs/auth
