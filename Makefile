.PHONY: setup up down logs build test integration lint dev-db dev-api dev-web backup
setup:
	node scripts/setup.mjs
up:
	docker compose up --build -d --wait
down:
	docker compose down
logs:
	docker compose logs -f app
build:
	npm run build
	go build ./cmd/server ./cmd/migrate
test:
	go test -race ./cmd/... ./internal/...
	npm run test:qr
integration:
	go test -race -tags=integration ./internal/...
lint:
	go vet ./cmd/... ./internal/...
	npm run lint
dev-db:
	docker compose -f compose.dev.yaml up -d --wait
dev-api:
	APP_URL=http://localhost:5173 HTTP_ADDR=127.0.0.1:8080 node scripts/run-server.mjs
dev-web:
	npm run dev -- --host localhost
backup:
	mkdir -p backups
	docker compose exec -T db pg_dump -U campus -d campus --format=custom > backups/campus-$$(date +%Y%m%d-%H%M%S).dump
