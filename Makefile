PROTOC_VERSION ?= 34.0
PROTOC_GEN_GO_VERSION ?= v1.36.11
PROTOC_GEN_GO_GRPC_VERSION ?= v1.6.1
PROTO_IMAGE ?= banka-4-backend-proto:protoc-$(PROTOC_VERSION)-go-$(PROTOC_GEN_GO_VERSION)-grpc-$(PROTOC_GEN_GO_GRPC_VERSION)
PROTO_FILES := $(wildcard common/proto/*.proto)
PROTO_GENERATED_FILES := common/pkg/pb

.PHONY: docker-up-build docker-up docker-down docker-down-rm-vol format swagger-docs proto proto-docker proto-image internal-proto proto-check test test-integration saga-e2e-up saga-e2e-down test-saga-e2e coverage-profile coverage coverage-report coverage-html

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
	cd services/email-service && swag init -g cmd/main.go -d ./,../../common
	cd services/interbank-service && swag init -g cmd/main.go -d ./,../../common

proto: proto-docker

proto-docker: proto-image
	docker run --rm \
		--user "$$(id -u):$$(id -g)" \
		-v "$(CURDIR):/workspace" \
		-w /workspace \
		$(PROTO_IMAGE) \
		make internal-proto

proto-image:
	docker build \
		-f docker/Dockerfile-proto \
		--build-arg PROTOC_VERSION=$(PROTOC_VERSION) \
		--build-arg PROTOC_GEN_GO_VERSION=$(PROTOC_GEN_GO_VERSION) \
		--build-arg PROTOC_GEN_GO_GRPC_VERSION=$(PROTOC_GEN_GO_GRPC_VERSION) \
		-t $(PROTO_IMAGE) \
		.

internal-proto:
	protoc --proto_path=. --go_out=. --go-grpc_out=. common/proto/*.proto

proto-check: proto-docker
	git diff --exit-code -- $(PROTO_FILES) $(PROTO_GENERATED_FILES)

test:
	go test ./common/... ./services/user-service/... ./services/banking-service/... ./services/trading-service/... ./services/email-service/... ./services/interbank-service/...

test-integration:
	go test -tags=integration ./common/... ./services/user-service/... ./services/banking-service/... ./services/trading-service/... ./services/email-service/... ./services/interbank-service/...

# SAGA chaos suite (SG-09..SG-11): needs the dev stack running with the saga
# overlay (Toxiproxy between trading and banking + fault injection enabled).
# EMAIL_SERVICE_PORT is not in the sample .env, so default it here.
SAGA_COMPOSE = EMAIL_SERVICE_PORT=$${EMAIL_SERVICE_PORT:-8084} docker compose -f docker-compose-dev.yml -f docker-compose-saga-test.yml

saga-e2e-up:
	$(SAGA_COMPOSE) up -d --build

saga-e2e-down:
	$(SAGA_COMPOSE) down

# e2e is a standalone, test-only module (kept out of go.work so it never enters
# the production Docker build graph). Run it from its own directory with
# GOWORK=off so it resolves via its own go.mod replace directives.
test-saga-e2e:
	cd e2e && GOWORK=off go test -tags=saga_e2e ./... -count=1 -v -timeout 30m

# Packages excluded from coverage: infrastructure with no business logic
#   cmd, docs, config, seed, server, logging, db, pb, middleware, job - bootstrap/infra
#   grpc, client - thin wrappers around external service calls
COVERAGE_EXCLUDE = /(cmd|docs|config|seed|server|logging|db|pb|middleware|job|grpc|client)$$

# All service/common packages (for running tests)
ALL_PKGS = ./common/... ./services/user-service/... ./services/banking-service/... ./services/trading-service/... ./services/email-service/... ./services/interbank-service/...

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
