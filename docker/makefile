RACE ?= 1
TESTPKGS ?= ./internal/handlers
TESTPAT ?= . 						# regex de nomes de testes (ex.: TestCompanies_|TestCompanyByID_)
TESTFLAGS := -count=1
ifeq ($(RACE),1)
  TESTFLAGS += -race
endif

.PHONY: test test-norace test-handlers test-repo itest itest-all cover cover-html

# Usa gotestsum se existir (melhor legibilidade), se não cai para go test
test:
	@echo "==> Running unit tests on $(TESTPKGS) with pattern '$(TESTPAT)'"
	@if command -v gotestsum >/dev/null 2>&1; then \
	  gotestsum --format=short-verbose -- -v $(TESTFLAGS) $(TESTPKGS) -run '$(TESTPAT)'; \
	else \
	  go test -v $(TESTFLAGS) $(TESTPKGS) -run '$(TESTPAT)'; \
	fi

test-norace:
	$(MAKE) test RACE=0

# Atalhos úteis
test-handlers:
	$(MAKE) test TESTPKGS="./internal/handlers"

test-repo:
	$(MAKE) test TESTPKGS="./internal/repository"

cover:
	go test $(TESTFLAGS) -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out | tail -n 1

cover-html: cover
	go tool cover -html=coverage.out -o coverage.html
	@echo "Abra coverage.html no navegador"

# Integração (separando de unitários!)
itest:
	go test -tags=integration -v ./internal/repository -run TestCompanyRepository_Integration_CreateGetUpdateReplaceDelete -count=1

itest-all:
	go test -tags=integration -v ./internal/repository -run TestCompanyRepository_Integration_CreateGetUpdateReplaceDelete -count=1 && \
	go test -tags=integration -v ./internal/broker -run TestRabbitMQ_PublishAndConsume -count=1
