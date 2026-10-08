NAME   = flaggr
BINARY = terraform-provider-$(NAME)

# tfplugindocs version used to generate docs/ (CI uses the same one).
TFPLUGINDOCS_VERSION ?= v0.25.0
# govulncheck version for `make vuln` (CI uses the same one).
GOVULNCHECK_VERSION ?= v1.8.0

default: build

# Builds ./terraform-provider-flaggr (version "dev") for use with dev_overrides:
# see README.md. Release builds come only from the Release workflow.
build:
	go build -o $(BINARY)

fmt:
	gofmt -s -w .
	terraform fmt -recursive examples/

vet:
	go vet ./...

test:
	go test -count=1 -cover ./...

# Reports known vulnerabilities in code the provider calls.
vuln:
	go run golang.org/x/vuln/cmd/govulncheck@$(GOVULNCHECK_VERSION) ./...

# Acceptance tests create and delete real resources: see README.md.
testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 120m

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION) generate --provider-name $(NAME)

.PHONY: default build fmt vet test vuln testacc docs
