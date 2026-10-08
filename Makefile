NAME       = flaggr
BINARY     = terraform-provider-$(NAME)
VERSION   ?= 0.1.0
OS_ARCH    = $(shell go env GOOS)_$(shell go env GOARCH)
PLUGIN_DIR = $(HOME)/.terraform.d/plugins/registry.terraform.io/flaggr-dev/$(NAME)/$(VERSION)/$(OS_ARCH)

# tfplugindocs version used to generate docs/ (CI uses the same one).
TFPLUGINDOCS_VERSION ?= v0.25.0

default: build

build:
	go build -o $(BINARY)

# Installs the binary where Terraform looks for registry.terraform.io/flaggr-dev/flaggr
# version $(VERSION) before downloading it.
install: build
	mkdir -p $(PLUGIN_DIR)
	mv $(BINARY) $(PLUGIN_DIR)/$(BINARY)_v$(VERSION)

fmt:
	gofmt -s -w .
	terraform fmt -recursive examples/

vet:
	go vet ./...

test:
	go test -count=1 -cover ./...

# Acceptance tests create and delete real resources: see README.md.
testacc:
	TF_ACC=1 go test ./... -v -count=1 -timeout 120m

docs:
	go run github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@$(TFPLUGINDOCS_VERSION) generate --provider-name $(NAME)

.PHONY: default build install fmt vet test testacc docs
