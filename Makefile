.PHONY: build clean install test integration

SKILLS := okf-sqlite okf-mysql okf-postgresql okf-bigquery okf-fs okf-git okf-viz

# Executable suffix (.exe on Windows).
ifeq ($(OS),Windows_NT)
EXT := .exe
endif

# Build every skill plus the okf-mcp server as named binaries, in place. The
# integration tests in tests/ locate them at skills/<name>/<name> (+ .exe).
build:
	for s in $(SKILLS); do (cd skills/$$s && go build -o $$s$(EXT) .) || exit 1; done
	cd okf-mcp && go build -o okf-mcp$(EXT) .

# Run every module's unit tests: shared library, skills, and the server.
test:
	cd okf-go && go test ./...
	for s in $(SKILLS); do (cd skills/$$s && go test ./...) || exit 1; done
	cd okf-mcp && go test ./...

# Run the integration suite (run `make build` first; MySQL/PostgreSQL cases also
# need `docker-compose up` in tests/ — SQLite, fs, git, schema-contract, and
# okf-mcp discovery cases run without Docker).
integration:
	cd tests && go test ./...

# Remove built binaries from the skill directories and the server.
clean:
	for s in $(SKILLS); do rm -f skills/$$s/$$s skills/$$s/$$s.exe; done
	rm -f okf-mcp/okf-mcp okf-mcp/okf-mcp.exe

# Build and install all skills + the okf-mcp server (delegates to skills.sh).
install:
	sh ./skills.sh
