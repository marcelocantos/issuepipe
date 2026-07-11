# Standing invariants for bullseye_convergence / make bullseye.
.PHONY: bullseye test fmt vet build

bullseye: fmt vet test
	@test -z "$$(git status --porcelain)" || { echo "❌ dirty tree:"; git status --porcelain; exit 1; }
	@echo "✅ clean tree"

fmt:
	@gofmt -l . | grep . && exit 1 || echo "✅ fmt"

vet:
	@go vet ./...
	@echo "✅ vet"

test:
	@go test ./...
	@echo "✅ tests"

build:
	@mkdir -p bin
	CGO_ENABLED=1 go build -o bin/issuepipe ./cmd/issuepipe
