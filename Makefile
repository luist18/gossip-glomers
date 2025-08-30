SHELL := /bin/sh
.DEFAULT_GOAL := help

# Maelstrom
MAELSTROM_VERSION := 0.2.4
MAELSTROM_URL := https://github.com/jepsen-io/maelstrom/releases/download/v$(MAELSTROM_VERSION)/maelstrom.tar.bz2
MAELSTROM_DIR := ./bin/maelstrom
MAELSTROM_SHA256 := 301ec71d6b12af0d765edb413f5cf5aa1046b5609bd4e31376a0b549548e5799

BINARY := ./bin/gossip-glomers

.PHONY: help all maelstrom gossip-glomers clean maelstrom-clean

help:
	@printf "Targets:\n"
	@printf "  maelstrom         Download & verify Maelstrom into $(MAELSTROM_DIR)\n"
	@printf "  gossip-glomers    Build Go binary to $(BINARY)\n"
	@printf "  clean             Remove built Go binary\n"
	@printf "  maelstrom-clean   Remove Maelstrom directory to force re-install\n"

all: maelstrom gossip-glomers

maelstrom: $(MAELSTROM_DIR)/maelstrom

$(MAELSTROM_DIR)/maelstrom:
	@mkdir -p "$(MAELSTROM_DIR)"
	@tmp="$$(mktemp)"; \
	set -e; \
	curl -L "$(MAELSTROM_URL)" -o "$$tmp"; \
	if command -v sha256sum >/dev/null 2>&1; then \
	  echo "$(MAELSTROM_SHA256)  $$tmp" | sha256sum -c -; \
	else \
	  echo "$(MAELSTROM_SHA256)  $$tmp" | shasum -a 256 -c -; \
	fi; \
	tar -xj -C "$(MAELSTROM_DIR)" --strip-components=1 -f "$$tmp"; \
	rm -f "$$tmp"

maelstrom-clean:
	rm -rf "$(MAELSTROM_DIR)"

gossip-glomers: $(BINARY)

$(BINARY): $(shell find . -name '*.go')
	@mkdir -p "$(dir $(BINARY))"
	GOEXPERIMENT=jsonv2 go build -o "$(BINARY)" .

clean:
	rm -f "$(BINARY)"

.PHONY: test

# Extract first goal after 'test'
TEST_NAME := $(word 1,$(filter-out test,$(MAKECMDGOALS)))

test: maelstrom gossip-glomers
	@if [ -z "$(TEST_NAME)" ]; then \
		echo "Usage: make test <name>"; exit 2; \
	fi
	@MAELSTROM_DIR=$(MAELSTROM_DIR) BINARY=$(BINARY) ./scripts/run-test.sh $(TEST_NAME)

# Swallow the extra goal so `make test echo` doesn't try to make a target named 'echo'
$(TEST_NAME):
	@:
