.PHONY: build clean install test

SKILLS := okf-sqlite okf-mysql okf-postgresql okf-bigquery okf-fs okf-git okf-mcp okf-enrich

# Build every skill in place (for development).
build:
	for s in $(SKILLS); do (cd skills/$$s && go build ./...) || exit 1; done

# Run every skill module's unit tests, plus the shared library.
test:
	cd okf-go && go test ./...
	for s in $(SKILLS); do (cd skills/$$s && go test ./...) || exit 1; done

# Remove built binaries from the skill directories.
clean:
	for s in $(SKILLS); do rm -f skills/$$s/$$s skills/$$s/$$s.exe; done

# Build and install all skills (delegates to skills.sh).
install:
	sh ./skills.sh
