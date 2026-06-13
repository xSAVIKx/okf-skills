.PHONY: build clean install test

SKILLS := okf-sqlite okf-mysql okf-postgresql okf-bigquery okf-fs okf-git okf-enrich

# Build every skill plus the okf-mcp server (for development).
build:
	for s in $(SKILLS); do (cd skills/$$s && go build ./...) || exit 1; done
	cd okf-mcp && go build ./...

# Run every module's unit tests: shared library, skills, and the server.
test:
	cd okf-go && go test ./...
	for s in $(SKILLS); do (cd skills/$$s && go test ./...) || exit 1; done
	cd okf-mcp && go test ./...

# Remove built binaries from the skill directories and the server.
clean:
	for s in $(SKILLS); do rm -f skills/$$s/$$s skills/$$s/$$s.exe; done
	rm -f okf-mcp/okf-mcp okf-mcp/okf-mcp.exe

# Build and install all skills + the okf-mcp server (delegates to skills.sh).
install:
	sh ./skills.sh
