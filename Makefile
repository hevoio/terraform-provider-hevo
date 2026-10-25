default: install

.PHONY: build install test testacc lint docs

build:
	go build ./...

## install: put the provider in $(go env GOPATH)/bin for a dev_overrides block in ~/.terraformrc
install:
	go install .

test:
	go test -race -shuffle=on ./...

## testacc: acceptance tests against a real account; needs HEVO_ACCESS_KEY, HEVO_SECRET_KEY and HEVO_HOST or HEVO_REGION
testacc:
	TF_ACC=1 go test -count=1 -timeout 120m ./hevo/tests/e2e/...

lint:
	golangci-lint run ./...

docs:
	go generate ./...
