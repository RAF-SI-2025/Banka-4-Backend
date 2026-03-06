.PHONY: proto

proto:
	 protoc --go_out=. --go-grpc_out=. proto/*/*.proto

docker-up:
	docker compose up --build

docker-down:
	docker compose down
