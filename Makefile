BIN     := git2
VERSION := 0.6.0
DIST    := dist

# where `make install` puts the binary (override: make install PREFIX=~/.local)
PREFIX  ?= /usr/local

.PHONY: build install uninstall release clean

build:
	go build -trimpath -ldflags "-s -w" -o $(BIN) .

install: build
	install -d $(PREFIX)/bin
	install -m 0755 $(BIN) $(PREFIX)/bin/$(BIN)
	@echo "installed $(PREFIX)/bin/$(BIN) — run: git2"

uninstall:
	rm -f $(PREFIX)/bin/$(BIN)

# cross-compile release binaries for all supported platforms
release:
	rm -rf $(DIST) && mkdir -p $(DIST)
	GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-macos-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-macos-amd64 .
	GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-windows-amd64.exe .
	cd $(DIST) && shasum -a 256 * > checksums.txt
	@ls -lh $(DIST)

clean:
	rm -rf $(BIN) $(BIN)-bin $(DIST)
