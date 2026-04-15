#!/usr/bin/env bash
set -euo pipefail

# Minimum regression script for infracast (single-cloud, AliCloud path)
# Run this before pushing to verify the core health of the repo.

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASS=0
FAIL=0
WARN=0

run_step() {
    local name="$1"
    shift
    echo ""
    echo "▶ $name"
    if "$@"; then
        echo -e "${GREEN}✓ PASS${NC}: $name"
        ((PASS+=1))
    else
        echo -e "${RED}✗ FAIL${NC}: $name"
        ((FAIL+=1))
    fi
}

warn_step() {
    local name="$1"
    shift
    echo ""
    echo "▶ $name"
    if "$@"; then
        echo -e "${GREEN}✓ PASS${NC}: $name"
        ((PASS+=1))
    else
        echo -e "${YELLOW}⚠ WARN${NC}: $name"
        ((WARN+=1))
    fi
}

check_fmt() {
    local files
    files=$(gofmt -l .)
    if [ -n "$files" ]; then
        echo "Unformatted files (gofmt -l .):"
        echo "$files"
        return 1
    fi
    echo "All Go files are formatted."
    return 0
}

check_vet() {
    go vet ./...
}

check_test() {
    go test -race ./...
}

check_build() {
    make build
}

check_version() {
    ./bin/infracast version
}

# Main
echo "═══════════════════════════════════════"
echo "  infracast Regression Suite"
echo "═══════════════════════════════════════"
echo "Go version: $(go version)"
echo ""

warn_step "Format check (gofmt)" check_fmt
run_step "Static analysis (go vet)" check_vet
run_step "Unit + integration tests (go test -race ./...)" check_test
run_step "Binary build (make build)" check_build
run_step "Binary smoke test (./bin/infracast version)" check_version

echo ""
echo "═══════════════════════════════════════"
echo "  Regression Summary"
echo "═══════════════════════════════════════"
echo -e "  Passed: ${GREEN}$PASS${NC}"
echo -e "  Failed: ${RED}$FAIL${NC}"
if [ "$WARN" -gt 0 ]; then
    echo -e "  Warned: ${YELLOW}$WARN${NC}"
fi

if [ "$FAIL" -gt 0 ]; then
    echo ""
    echo -e "${RED}Regression FAILED.${NC} Fix the errors above before pushing."
    exit 1
fi

echo ""
echo -e "${GREEN}All regression checks passed.${NC}"
exit 0
