default: fmt lint install generate

ACC_TEST_TIMEOUT ?= 120m
ACC_TEST_PACKAGES ?= ./...
ACC_TEST_CATEGORY_PACKAGES ?= ./internal/provider/resources/...
ACC_TEST_SCENARIO_PACKAGES ?= ./internal/provider/acctest
ACC_TEST_RESOURCES_RE ?= ^TestAcc(Certificate|Group|GroupRole|GroupUser|Invite|OrganizationDataRetention|OrganizationSpendAlert|OrganizationUser|Project|ProjectGroup|ProjectGroupRole|ProjectModelPermissions|ProjectRateLimit|ProjectRole|ProjectSpendAlert|ProjectUserRole|UserRole)_
ACC_TEST_DATASOURCES_RE ?= ^TestAccDataSource
ACC_TEST_SCENARIOS_RE ?= ^TestAccScenario_

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=10 ./...

testacc:
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) $(ACC_TEST_PACKAGES)

testacc-resources:
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) -run '$(ACC_TEST_RESOURCES_RE)' $(ACC_TEST_CATEGORY_PACKAGES)

testacc-datasources:
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) -run '$(ACC_TEST_DATASOURCES_RE)' $(ACC_TEST_CATEGORY_PACKAGES)

testacc-data-sources: testacc-datasources

testacc-scenarios:
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) -run '$(ACC_TEST_SCENARIOS_RE)' $(ACC_TEST_SCENARIO_PACKAGES)

.PHONY: fmt lint test testacc testacc-resources testacc-datasources testacc-data-sources testacc-scenarios build install generate
