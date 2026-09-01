.PHONY: dev build run test docker-up docker-down clean

# Run backend development server
dev-backend:
	go run ./cmd/server

# Run frontend development server
dev-frontend:
	cd web && npm run dev

# Build both frontend and backend
build:
	cd web && npm ci && npm run build
	go build -ldflags="-w -s" -o bin/hephaestus ./cmd/server

# Run application locally
run: build
	./bin/hephaestus

# Docker operations
docker-up:
	docker compose up -d --build

docker-down:
	docker compose down

clean:
	rm -rf bin/ web/dist/ logs/*.log
