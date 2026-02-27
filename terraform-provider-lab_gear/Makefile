# Strip any leading v so both 'v0.1.0' and '0.1.0' work as VERSION input.
VERSION ?= $(shell git describe --tags --abbrev=0 2>/dev/null || echo "v0.0.0-dev")
override VERSION := $(shell echo "$(VERSION)" | sed 's/^v//')

BINARY := terraform-provider-lab_gear
DIST   := dist

.PHONY: build install release clean test

# Local build — produces ./terraform-provider-lab_gear
build:
	go build -o $(BINARY) .

# Install to GOPATH/bin with the correct binary name for dev_overrides.
# In ~/.terraformrc:
#   provider_installation {
#     dev_overrides { "registry.terraform.io/tphummel/lab_gear" = "<GOPATH>/bin" }
#     direct {}
#   }
install:
	go build -o "$(shell go env GOPATH)/bin/$(BINARY)" .

# Cross-compile and package for GitHub Releases (filesystem mirror format).
# Produces dist/terraform-provider-lab_gear_VERSION_OS_ARCH.zip + SHA256SUMS.
release: clean
	mkdir -p $(DIST)
	@for target in \
		"linux   amd64" \
		"linux   arm64" \
		"darwin  amd64" \
		"darwin  arm64"; do \
		os=$$(echo $$target | awk '{print $$1}'); \
		arch=$$(echo $$target | awk '{print $$2}'); \
		echo "Building $${os}/$${arch}..."; \
		GOOS=$$os GOARCH=$$arch CGO_ENABLED=0 \
			go build -ldflags="-s -w" -o $(DIST)/$(BINARY)_v$(VERSION) .; \
		cd $(DIST) && zip $(BINARY)_$(VERSION)_$${os}_$${arch}.zip $(BINARY)_v$(VERSION) \
			&& rm $(BINARY)_v$(VERSION) && cd ..; \
	done
	cd $(DIST) && sha256sum $(BINARY)_$(VERSION)_*.zip > $(BINARY)_$(VERSION)_SHA256SUMS

clean:
	rm -rf $(DIST) $(BINARY)

test:
	go test ./...
