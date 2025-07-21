.DEFAULT_GOAL = all
GO := go

.PHONY: all
all: tidy docs generate lint vet test

.PHONY: check
check: generate vet

.PHONY: tidy
tidy:
	./scripts/run.sh '*' go mod tidy

.PHONY: docs
docs:
	./scripts/docs.sh

.PHONY: generate
generate:
	./scripts/run.sh '*' go generate ./...

.PHONY: lint
lint:
	./scripts/run.sh -v 'examples' check-copyright
	./scripts/run.sh -v 'examples' check-large-files
	./scripts/run.sh -v 'examples' check-imports ./...
	./scripts/run.sh -v 'examples' check-atomic-align ./...
	./scripts/run.sh -v 'examples' staticcheck ./...
	./scripts/run.sh -v 'examples' golangci-lint run

.PHONY: vet
vet:
	./scripts/run.sh '*' go vet ./...
	GOOS=linux   GOARCH=386   ./scripts/run.sh '*' go vet ./...
	GOOS=linux   GOARCH=amd64 ./scripts/run.sh '*' go vet ./...
	GOOS=linux   GOARCH=arm   ./scripts/run.sh '*' go vet ./...
	GOOS=linux   GOARCH=arm64 ./scripts/run.sh '*' go vet ./...
	GOOS=windows GOARCH=386   ./scripts/run.sh '*' go vet ./...
	GOOS=windows GOARCH=amd64 ./scripts/run.sh '*' go vet ./...
	GOOS=windows GOARCH=arm64 ./scripts/run.sh '*' go vet ./...
	GOOS=darwin  GOARCH=amd64 ./scripts/run.sh '*' go vet ./...
	GOOS=darwin  GOARCH=arm64 ./scripts/run.sh '*' go vet ./...

.PHONY: test
test:
	./scripts/run.sh '*'           go test ./...              -race -count=1 -bench=. -benchtime=1x
	./scripts/run.sh 'integration' go test ./... -tags=gogo   -race -count=1 -bench=. -benchtime=1x
	./scripts/run.sh 'integration' go test ./... -tags=custom -race -count=1 -bench=. -benchtime=1x

.PHONY: gen-bazel
gen-bazel:
	@echo "Generating WORKSPACE"
	@echo 'workspace(name = "io_storj_drpc")' > WORKSPACE
	@echo 'Running gazelle...'
	${GO} run github.com/bazelbuild/bazel-gazelle/cmd/gazelle@v0.40.0 \
		update --go_prefix=storj.io/drpc --exclude=examples --exclude=scripts --repo_root=.
	@echo 'You should now be able to build Cockroach using:'
	@echo '  ./dev build short -- --override_repository=io_storj_drpc=${CURDIR}'

.PHONY: clean-bazel
clean-bazel:
	git clean -dxf WORKSPACE BUILD.bazel '**/BUILD.bazel'
