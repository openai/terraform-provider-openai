default: fmt lint install generate

ACC_TEST_TIMEOUT ?= 120m
ACC_TEST_PACKAGES ?= ./...
ACC_TEST_CATEGORY_PACKAGES ?= ./internal/provider/resources/...
ACC_TEST_SCENARIO_PACKAGES ?= ./internal/provider/acctest
ACC_TEST_RESOURCES_RE ?= ^TestAcc(Certificate|Group|GroupRole|GroupUser|Invite|OrganizationDataRetention|OrganizationSpendAlert|OrganizationUser|Project|ProjectGroup|ProjectGroupRole|ProjectModelPermissions|ProjectRateLimit|ProjectRole|ProjectSpendAlert|ProjectUser|ProjectUserRole|UserRole)_
ACC_TEST_DATASOURCES_RE ?= ^TestAccDataSource
ACC_TEST_SCENARIOS_RE ?= ^TestAccScenario_

ifndef TF_ACC_TERRAFORM_VERSION
TF_ACC_TERRAFORM_PATH ?= $(shell command -v terraform 2>/dev/null)
endif
export TF_ACC_TERRAFORM_PATH

build:
	go build -v ./...

install: build
	go install -v ./...

lint:
	golangci-lint run

generate:
	@command -v terraform >/dev/null 2>&1 || { \
		echo "Terraform CLI not found; install Terraform before generating documentation."; \
		exit 1; \
	}
	cd tools; go generate ./...

fmt:
	gofmt -s -w -e .

check-terraform:
	@if [ -n "$$TF_ACC_TERRAFORM_PATH" ]; then \
		test -x "$$TF_ACC_TERRAFORM_PATH" || { \
			echo "Terraform CLI is not executable: $$TF_ACC_TERRAFORM_PATH"; \
			exit 1; \
		}; \
	elif [ -z "$$TF_ACC_TERRAFORM_VERSION" ]; then \
		echo "Terraform CLI not found; install Terraform or set TF_ACC_TERRAFORM_PATH or TF_ACC_TERRAFORM_VERSION."; \
		exit 1; \
	fi

test: check-terraform
	go test -v -cover -timeout=120s -parallel=10 ./... \
		./.github/actions/setup-terraform \
		./.github/actions/verify-release-sboms

testacc: check-terraform
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) $(ACC_TEST_PACKAGES)

testacc-resources: check-terraform
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) -run '$(ACC_TEST_RESOURCES_RE)' $(ACC_TEST_CATEGORY_PACKAGES)

testacc-datasources: check-terraform
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) -run '$(ACC_TEST_DATASOURCES_RE)' $(ACC_TEST_CATEGORY_PACKAGES)

testacc-data-sources: testacc-datasources

testacc-scenarios: check-terraform
	TF_ACC=1 go test -v -cover -timeout $(ACC_TEST_TIMEOUT) -run '$(ACC_TEST_SCENARIOS_RE)' $(ACC_TEST_SCENARIO_PACKAGES)

.PHONY: check-terraform fmt lint test testacc testacc-resources testacc-datasources testacc-data-sources testacc-scenarios build install generate
