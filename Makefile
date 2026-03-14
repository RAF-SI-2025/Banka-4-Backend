docker-up-build:
	docker compose -f docker-compose-dev.yml up --build

docker-up:
	docker compose -f docker-compose-dev.yml up

docker-down:
	docker compose -f docker-compose-dev.yml down

docker-down-rm-vol:
	docker compose -f docker-compose-dev.yml down -v

swagger-docs:
	cd services/user-service && swag init -g cmd/main.go -d ./,../../common
