VERSION := $(shell sed -n 's/^const version = "\(.*\)"/\1/p' main.go)
DOCKER := docker run --rm -v "$(CURDIR)":/src:ro -v "$(CURDIR)/dist":/dist -e GOCACHE=/tmp/cache
LDFLAGS := -s -w

.PHONY: all test test-alt windows vista macos linux checksums clean

all: test windows vista macos linux checksums

test:
	go vet ./...
	go test -race ./...

test-alt:
	docker run --rm -v "$(CURDIR)":/go/src/replado-connector:ro -w /go/src/replado-connector -e GO111MODULE=off -e GOCACHE=/tmp/cache golang:1.10 sh -c 'go vet . && go test . && GOOS=windows GOARCH=386 go vet .'
	docker run --rm -v "$(CURDIR)":/src:ro -w /src -e GOCACHE=/tmp/cache golang:1.20 sh -c 'go vet ./... && go test ./... && GOOS=windows GOARCH=386 go vet ./...'

dist:
	mkdir -p dist

windows: dist
	$(DOCKER) -w /src golang:1.20 sh -c 'GOOS=windows GOARCH=386 go build -trimpath -ldflags "-H=windowsgui $(LDFLAGS)" -o /dist/Replado-Connector.exe .'

vista: dist
	docker run --rm -v "$(CURDIR)":/go/src/replado-connector:ro -v "$(CURDIR)/dist":/dist -w /go/src/replado-connector -e GO111MODULE=off -e GOCACHE=/tmp/cache golang:1.10 sh -c 'GOOS=windows GOARCH=386 go build -ldflags "-H=windowsgui $(LDFLAGS)" -o /dist/Replado-Connector-Vista.exe .'

macos: dist
	$(DOCKER) -w /src golang:1.20 sh -c 'GOOS=darwin GOARCH=amd64 go build -trimpath -ldflags "$(LDFLAGS)" -o /dist/macos-amd64 . && GOOS=darwin GOARCH=arm64 go build -trimpath -ldflags "$(LDFLAGS)" -o /dist/macos-arm64 .'
	cd dist && lipo -create -output replado-connector macos-amd64 macos-arm64 && rm macos-amd64 macos-arm64
	cd dist && zip -q -X Replado-Connector-macOS.zip replado-connector && rm replado-connector

linux: dist
	$(DOCKER) -w /src golang:1.20 sh -c 'for a in amd64 arm64; do CGO_ENABLED=0 GOOS=linux GOARCH=$$a go build -trimpath -ldflags "$(LDFLAGS)" -o /dist/linux-$$a/replado-connector . ; done'
	cd dist && for a in amd64 arm64; do tar -czf replado-connector-linux-$$a.tar.gz -C linux-$$a replado-connector && rm -r linux-$$a; done

checksums:
	cd dist && shasum -a 256 Replado-Connector.exe Replado-Connector-Vista.exe Replado-Connector-macOS.zip replado-connector-linux-*.tar.gz > SHA256SUMS.txt

clean:
	rm -rf dist
