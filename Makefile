BIN     := git2
VERSION := 0.2.0
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
	GOOS=darwin  GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-$(VERSION)-macos-arm64 .
	GOOS=darwin  GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-$(VERSION)-macos-intel .
	GOOS=linux   GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-$(VERSION)-linux-amd64 .
	GOOS=linux   GOARCH=arm64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-$(VERSION)-linux-arm64 .
	GOOS=windows GOARCH=amd64 go build -trimpath -ldflags "-s -w" -o $(DIST)/$(BIN)-$(VERSION)-windows-amd64.exe .
	@ls -lh $(DIST)

clean:
	rm -rf $(BIN) $(BIN)-bin $(DIST)
