.PHONY: build run clean skills-build

build:
	cd app && go build -o ../okf-reference-agent.exe main.go

run:
	cd app && go run main.go

clean:
	del /q okf-reference-agent.exe 2>nul || true
	cd ../skills/okf-sqlite && del /q okf-sqlite.exe 2>nul || true
	cd ../skills/okf-mysql && del /q okf-mysql.exe 2>nul || true
	cd ../skills/okf-postgresql && del /q okf-postgresql.exe 2>nul || true
	cd ../skills/okf-bigquery && del /q okf-bigquery.exe 2>nul || true

skills-build:
	cd ../skills/okf-sqlite && go build -o okf-sqlite.exe main.go
	cd ../skills/okf-mysql && go build -o okf-mysql.exe main.go
	cd ../skills/okf-postgresql && go build -o okf-postgresql.exe main.go
	cd ../skills/okf-bigquery && go build -o okf-bigquery.exe main.go
