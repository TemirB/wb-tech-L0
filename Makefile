# Makefile for WB Tech L0 project

.PHONY: up down build start stop restart logs stats spam spam-stop spam-stats clean

# Docker compose file location
COMPOSE_FILE = docker/docker-compose.yml
ENV_FILE = env/.env

# Default target
all: up

# Start all services
up:
	docker-compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) up -d

# Start with build
build:
	docker-compose -f $(COMPOSE_FILE) --env-file $(ENV_FILE) up -d --build

# Stop all services
down:
	docker-compose -f $(COMPOSE_FILE) down

# Stop and remove volumes
clean:
	docker-compose -f $(COMPOSE_FILE) down -v

# Restart services
restart: down up

# View logs
logs:
	docker-compose -f $(COMPOSE_FILE) logs -f

# View specific service logs
logs-app:
	docker-compose -f $(COMPOSE_FILE) logs -f app

logs-kafka:
	docker-compose -f $(COMPOSE_FILE) logs -f kafka

logs-postgres:
	docker-compose -f $(COMPOSE_FILE) logs -f postgres

logs-spammer:
	docker-compose -f $(COMPOSE_FILE) logs -f spammer

# Check service status
ps:
	docker-compose -f $(COMPOSE_FILE) ps

# Health check
health:
	curl -f http://localhost:8081/health || echo "App is not healthy"

# Spammer control
spam:
	curl -X POST http://localhost:8082/start \
  -H "Content-Type: application/json" \
  -d '{"rate": 50,"duration": "30s"}'

spam-fast:
	curl -X POST http://localhost:8082/start \
  -H "Content-Type: application/json" \
  -d '{"rate": 100,"duration": "100s"}'

spam-heavy:
	curl -X POST http://localhost:8082/start \
  -H "Content-Type: application/json" \
  -d '{"rate": 200,"duration": "60s"}'

spam-stop:
	curl -X POST http://localhost:8082/stop

spam-stats:
	curl http://localhost:8082/stats

# Database operations
db-shell:
	docker-compose -f $(COMPOSE_FILE) exec postgres psql -U app -d orders

db-backup:
	docker-compose -f $(COMPOSE_FILE) exec postgres pg_dump -U app -d orders > backup_$(shell date +%Y%m%d_%H%M%S).sql

# Kafka operations
kafka-topics:
	docker-compose -f $(COMPOSE_FILE) exec kafka kafka-topics.sh --bootstrap-server localhost:9092 --list

kafka-consumers:
	docker-compose -f $(COMPOSE_FILE) exec kafka kafka-consumer-groups.sh --bootstrap-server localhost:9092 --list

# Application testing
test-order:
	curl http://localhost:8081/order/b563feb7b2b84b6test

test-health:
	curl http://localhost:8081/health

# Build specific services
build-app:
	docker-compose -f $(COMPOSE_FILE) build app

build-spammer:
	docker-compose -f $(COMPOSE_FILE) build spammer

# Show help
help:
	@echo "Available commands:"
	@echo "  make up           - Start all services"
	@echo "  make down         - Stop all services"
	@echo "  make build        - Build and start services"
	@echo "  make restart      - Restart services"
	@echo "  make logs         - View logs of all services"
	@echo "  make logs-app     - View app logs"
	@echo "  make logs-kafka   - View kafka logs"
	@echo "  make ps           - Check service status"
	@echo "  make health       - Health check"
	@echo "  make spam         - Start spam test (50 msg/s, 30s)"
	@echo "  make spam-fast    - Fast spam test (100 msg/s, 10s)"
	@echo "  make spam-heavy   - Heavy spam test (200 msg/s, 60s)"
	@echo "  make spam-stop    - Stop spam test"
	@echo "  make spam-stats   - Show spam statistics"
	@echo "  make clean        - Stop and remove volumes"
	@echo "  make help         - Show this help"