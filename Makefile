.PHONY: up down build test smoke logs clean free-ports

up: free-ports
	docker compose up --build -d

free-ports:
	@-docker compose down 2>/dev/null; true
	@for port in 3000 8080; do \
		pid=$$(lsof -ti tcp:$$port 2>/dev/null); \
		if [ -n "$$pid" ]; then \
			name=$$(ps -p $$pid -o comm= 2>/dev/null); \
			if echo "$$name" | grep -qiE 'docker|com.docker'; then \
				echo "Skipping port $$port — held by Docker ($$name)"; \
			else \
				echo "Killing $$name ($$pid) on port $$port"; \
				kill -9 $$pid; \
			fi; \
		fi; \
	done

down:
	docker compose down

build:
	docker compose build

test:
	docker run --rm -v "$(shell pwd)/control-plane":/app -w /app -e GOTOOLCHAIN=auto golang:1.23-alpine \
		sh -c "go vet ./... && go test ./..."

smoke:
	bash test/smoke.sh

logs:
	docker compose logs -f

clean:
	docker compose down --volumes --remove-orphans

grafana:
	docker compose -f docker-compose.yml -f docker-compose.override.yml up --build -d
