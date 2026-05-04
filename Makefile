docker-up-build:
	docker compose -f docker-compose-dev.yml up --build

docker-up:
	docker compose -f docker-compose-dev.yml up

docker-down:
	docker compose -f docker-compose-dev.yml down

docker-down-rm-vol:
	docker compose -f docker-compose-dev.yml down -v

format:
	gofmt -w .

swagger-docs:
	cd services/user-service && swag init -g cmd/main.go -d ./,../../common
	cd services/banking-service && swag init -g cmd/main.go -d ./,../../common
	cd services/trading-service && swag init -g cmd/main.go -d ./,../../common

proto:
	protoc --proto_path=. --go_out=. --go-grpc_out=. common/proto/*.proto

test:
	go test ./common/... ./services/user-service/... ./services/banking-service/... ./services/trading-service/...

test-integration:
	go test -tags=integration ./common/... ./services/user-service/... ./services/banking-service/... ./services/trading-service/...

# Packages excluded from coverage: infrastructure with no business logic
#   cmd, docs, config, seed, server, logging, db, pb, middleware, job - bootstrap/infra
#   grpc, client - thin wrappers around external service calls
COVERAGE_EXCLUDE = /(cmd|docs|config|seed|server|logging|db|pb|middleware|job|grpc|client)$$

# All service/common packages (for running tests)
ALL_PKGS = ./common/... ./services/user-service/... ./services/banking-service/... ./services/trading-service/...

coverage-profile:
	mkdir -p .tmp-coverage
	go test -count=1 -v -tags=integration -covermode=count \
		-coverpkg=$$(go list $(ALL_PKGS) \
			| grep -v '/internal/integration_test$$' \
			| grep -vE '$(COVERAGE_EXCLUDE)' \
			| paste -sd, -) \
		-coverprofile=.tmp-coverage/coverage.out \
		$(ALL_PKGS)

coverage: coverage-profile
	@go tool cover -func=.tmp-coverage/coverage.out | tail -n 1

coverage-report: coverage-profile
	@echo "=== Coverage by layer ==="
	@echo "--- Services ---"
	@go tool cover -func=.tmp-coverage/coverage.out | grep '/service/' | grep -v 'total:' | awk '{gsub(/%/,"",$$NF); sum+=$$NF; n++} END {printf "  service:    %.1f%% (%d funcs)\n", sum/n, n}'
	@echo "--- Handlers ---"
	@go tool cover -func=.tmp-coverage/coverage.out | grep '/handler/' | grep -v 'total:' | awk '{gsub(/%/,"",$$NF); sum+=$$NF; n++} END {printf "  handler:    %.1f%% (%d funcs)\n", sum/n, n}'
	@echo "--- Repositories ---"
	@go tool cover -func=.tmp-coverage/coverage.out | grep '/repository/' | grep -v 'total:' | awk '{gsub(/%/,"",$$NF); sum+=$$NF; n++} END {printf "  repository: %.1f%% (%d funcs)\n", sum/n, n}'
	@echo "--- Common ---"
	@go tool cover -func=.tmp-coverage/coverage.out | grep 'common/pkg/' | grep -v 'total:' | awk '{gsub(/%/,"",$$NF); sum+=$$NF; n++} END {printf "  common:     %.1f%% (%d funcs)\n", sum/n, n}'
	@echo ""
	@echo "=== Total (statement-weighted) ==="
	@go tool cover -func=.tmp-coverage/coverage.out | tail -n 1

coverage-html: coverage-profile
	go tool cover -html=.tmp-coverage/coverage.out
