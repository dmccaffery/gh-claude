# Copyright 2026 Bitwise Media Group Ltd.
# SPDX-License-Identifier: MIT

# Developer tasks. `make help` lists targets; `make pr` is the full local gate, and
# `make ci` is exactly what the reusable CI workflow runs (lint, test, build).
#
# The Go developer CLIs (addlicense, golangci-lint, govulncheck, gotestsum,
# gocover-cobertura, goreleaser, syft) are pinned in tools/go.mod — a separate
# module so their dependency graphs never touch the application's go.mod — and
# invoked with `go tool -modfile=tools/go.mod <name>`: compiled into the build cache
# on first use, no GOBIN, no binaries to manage. -modfile anchors on the root go.mod
# and runs the tool in the current directory, so relative paths just work.

# one -ignore flag per non-empty line in .licenseignore (quoted to avoid shell globbing)
LICENSE_HOLDER := 'Bitwise Media Group Ltd.'
LICENSE_IGNORE := $(foreach pattern,$(shell cat .licenseignore 2>/dev/null),-ignore '$(pattern)')

APP     := gh-claude
APP_PKG := .
TOOL    := go tool -modfile=tools/go.mod
# Injected into main.version so `gh claude version` and the integrity check know
# the build. A local build reports a git describe (not a bare semver), which the
# integrity check treats as an un-gated dev build; releases inject the real tag
# via GoReleaser (see .goreleaser.yaml).
VERSION := $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)

# Security-policy signing (see docs/security-policy.md). The policy is signed with
# an OpenSSH signature (`ssh-keygen -Y sign`) so the key can be a FIDO2
# sk-ssh-ed25519 YubiKey. POLICY_NS must match policyNamespace in
# internal/integrity/signature.go. Override the rest per invocation, e.g.:
#   make policy-sign   POLICY_KEY=~/.ssh/id_ed25519_sk
#   make policy-verify ALLOWED_SIGNERS=policy.allowed_signers SIGNER=policy@bitwise
POLICY          ?= policy.json
POLICY_SIG       = $(POLICY).sig
POLICY_NS       := gh-claude-policy
POLICY_KEY      ?=
ALLOWED_SIGNERS ?= policy.allowed_signers
SIGNER          ?=

.DEFAULT_GOAL := help

.PHONY: help
help: ## List available targets
	@ grep -hE '^[a-zA-Z_-]+:.*?## ' $(MAKEFILE_LIST) \
		| awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-12s\033[0m %s\n", $$1, $$2}'

.PHONY: pr
pr: tidy license fmt lint test build ## full local gate before opening a pull request

.PHONY: ci
ci: lint test build ## the gates the reusable CI workflow runs

.PHONY: tidy
tidy: ## tidy the app and tools module graphs
	@ rm -f go.sum; go mod tidy
	@ go -C tools mod tidy

.PHONY: fmt
fmt: ## format the code and inject license headers
	@ go fmt ./...
	@ $(TOOL) golangci-lint run --fix
	@ $(TOOL) addlicense -l mit -c $(LICENSE_HOLDER) -s=only $(LICENSE_IGNORE) .

.PHONY: license
license: ## inject SPDX license headers (addlicense)
	@ $(TOOL) addlicense -l mit -c $(LICENSE_HOLDER) -s=only $(LICENSE_IGNORE) .

.PHONY: lint
lint: ## run all check-mode static analysis (addlicense, golangci-lint, govulncheck)
	@ $(TOOL) addlicense -l mit -c $(LICENSE_HOLDER) -s=only $(LICENSE_IGNORE) -check .
	@ $(TOOL) golangci-lint run
	@ $(TOOL) govulncheck ./...

.PHONY: link
link: build ## install the local copy of gh-claude as an extension
	@ gh extensions remove claude 1>/dev/null 2>&1 || true
	@ gh extensions install .

# -covermode=atomic is the race-safe counter mode `-race` requires. gotestsum runs
# the suite and writes a JUnit report in one pass (propagating the test exit code,
# which a bare `go test | …` pipe would swallow); gocover-cobertura turns the profile
# into Cobertura XML. Coverage (cobertura-coverage.xml) and test results (junit.xml)
# land in coverage/ where the reusable CI workflow uploads them to Codecov.
.PHONY: test
test: ## run the unit tests with coverage (coverage/ → Codecov in CI)
	@ mkdir -p coverage
	@ $(TOOL) gotestsum --junitfile coverage/junit.xml -- \
		-race -covermode=atomic -coverprofile=coverage/coverage.out ./...
	@ $(TOOL) gocover-cobertura <coverage/coverage.out >coverage/cobertura-coverage.xml

.PHONY: build
build: ## build the extension binary (./gh-claude)
	@ CGO_ENABLED=0 go build -trimpath -ldflags "-s -w -X main.version=$(VERSION)" -o $(APP) $(APP_PKG)

.PHONY: install
install: build ## install the local build as a gh extension for end-to-end testing
	@ gh extension install . 2>/dev/null || gh extension upgrade gh-claude

# --skip=sign: cosign keyless signing needs the GitHub Actions OIDC token, so it
# only works in the release workflow — locally it would fail or prompt.
.PHONY: snapshot
snapshot: ## build a local release snapshot (binaries + SBOMs, no publish or signing)
	@ $(TOOL) goreleaser release --snapshot --clean --skip=sign

.PHONY: release
release: ## build and publish a release (needs a vX.Y.Z tag + creds)
	@ $(TOOL) goreleaser release --clean

# Touch the YubiKey when it blinks. `ssh-keygen -Y sign` writes $(POLICY).sig.
.PHONY: policy-sign
policy-sign: ## sign $(POLICY) with an SSH key (POLICY_KEY=private key / FIDO2 key handle)
	@ command -v ssh-keygen >/dev/null || { echo "ssh-keygen not found" >&2; exit 1; }
	@ test -n "$(POLICY_KEY)" || { echo "set POLICY_KEY=<ssh private key or FIDO2 key handle>" >&2; exit 1; }
	@ test -f "$(POLICY)"     || { echo "no $(POLICY) (set POLICY=...)" >&2; exit 1; }
	@ ssh-keygen -Y sign -n $(POLICY_NS) -f "$(POLICY_KEY)" "$(POLICY)"
	@ echo "signed -> $(POLICY_SIG)  (check: make policy-verify; then publish both to policyURL)"

# Offline, hardware-free check (also CI-friendly): confirm the published signature
# verifies before clients ever fetch it. ALLOWED_SIGNERS holds one
# `<principal> <public-key>` line per policy key; SIGNER is the principal to match.
.PHONY: policy-verify
policy-verify: ## verify $(POLICY_SIG) against $(ALLOWED_SIGNERS) (SIGNER=principal)
	@ command -v ssh-keygen >/dev/null || { echo "ssh-keygen not found" >&2; exit 1; }
	@ test -f "$(ALLOWED_SIGNERS)" || { echo "no $(ALLOWED_SIGNERS): one '<principal> <public-key>' line per policy key" >&2; exit 1; }
	@ test -n "$(SIGNER)"          || { echo "set SIGNER=<principal listed in $(ALLOWED_SIGNERS)>" >&2; exit 1; }
	@ test -f "$(POLICY_SIG)"      || { echo "no $(POLICY_SIG) (run make policy-sign)" >&2; exit 1; }
	@ ssh-keygen -Y verify -f "$(ALLOWED_SIGNERS)" -I "$(SIGNER)" -n $(POLICY_NS) -s "$(POLICY_SIG)" <"$(POLICY)"
