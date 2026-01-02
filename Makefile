DB_DSN = "postgres://user:password@localhost:5432/$*_db?sslmode=disable"

.PHONY: db-up db-down help

db-up:
	docker-compose up -d

db-down:
	docker-compose down

gen-%:
	@echo "📍 Generating proto for $*..."
	protoc --go_out=. --go_opt=paths=source_relative \
		--go-grpc_out=. --go-grpc_opt=paths=source_relative \
		proto/$*/$*.proto

run-%:
	@echo "🚀 Running $* service..."
	cd services/$* && go run cmd/main.go

migrate-up-%:
	@echo "🐘 Applying migrations for $*_db..."
	goose -dir services/$*/migrations postgres $(DB_DSN) up

migrate-down-%:
	@echo "🔻 Rolling back migrations for $*_db..."
	goose -dir services/$*/migrations postgres $(DB_DSN) down

migration-new-%:
	@echo "📝 Creating new migration for $*..."
	goose -dir services/$*/migrations create $(name) sql

test-%:
	@echo "🔨 Testing $* service..."
	cd services/$*/tests && go test -v

gen-all: gen-auth gen-notification gen-payment