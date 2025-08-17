compose-up:
	docker compose -f docker/docker-compose.yml up --build -d

compose-down:
	docker compose -f docker/docker-compose.yml down -v

seed:
	./scripts/seed-order.sh

curl:
	curl -s http://localhost:8081/order'b563feb7b2b84b6test | jq .