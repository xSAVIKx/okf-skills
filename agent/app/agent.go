package main

import (
	"log"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
)

// BuildAgent constructs and configures the core LLM agent.
func BuildAgent(m model.LLM) (agent.Agent, error) {
	// Initialize individual SQLite tool definition
	sqliteTool, err := functiontool.New(
		functiontool.Config{
			Name:        "sqlite_connector",
			Description: "Runs produce or ingest actions on a SQLite database. Always use this tool when requested to document or sync comments for a SQLite database.",
		},
		runSqliteTool,
	)
	if err != nil {
		log.Fatalf("Failed to create sqlite tool: %v", err)
	}

	// Initialize individual MySQL tool definition
	mysqlTool, err := functiontool.New(
		functiontool.Config{
			Name:        "mysql_connector",
			Description: "Runs produce or ingest actions on a MySQL database. Always use this tool when requested to document or sync comments for a MySQL database.",
		},
		runMysqlTool,
	)
	if err != nil {
		log.Fatalf("Failed to create mysql tool: %v", err)
	}

	// Initialize individual PostgreSQL tool definition
	postgresqlTool, err := functiontool.New(
		functiontool.Config{
			Name:        "postgresql_connector",
			Description: "Runs produce or ingest actions on a PostgreSQL database. Always use this tool when requested to document or sync comments for a PostgreSQL database.",
		},
		runPostgresqlTool,
	)
	if err != nil {
		log.Fatalf("Failed to create postgresql tool: %v", err)
	}

	// Initialize individual BigQuery tool definition
	bigqueryTool, err := functiontool.New(
		functiontool.Config{
			Name:        "bigquery_connector",
			Description: "Runs produce or ingest actions on a BigQuery dataset. Always use this tool when requested to document or sync descriptions for a BigQuery dataset.",
		},
		runBigqueryTool,
	)
	if err != nil {
		log.Fatalf("Failed to create bigquery tool: %v", err)
	}

	// Initialize individual File System tool definition
	fsTool, err := functiontool.New(
		functiontool.Config{
			Name:        "fs_connector",
			Description: "Runs produce or ingest actions on a local directory structure. Always use this tool when requested to document or sync descriptions for a file system directory.",
		},
		runFsTool,
	)
	if err != nil {
		log.Fatalf("Failed to create fs tool: %v", err)
	}

	// Initialize individual Git tool definition
	gitTool, err := functiontool.New(
		functiontool.Config{
			Name:        "git_connector",
			Description: "Runs produce or ingest actions on a Git repository. Always use this tool when requested to document or sync descriptions for a Git repository.",
		},
		runGitTool,
	)
	if err != nil {
		log.Fatalf("Failed to create git tool: %v", err)
	}

	// Instantiate the core ADK LLM agent
	return llmagent.New(llmagent.Config{
		Name:        "okf-agent",
		Model:       m,
		Description: "An AI agent that creates and ingests Open Knowledge Format (OKF) bundles for database schemas, filesystems, and git repositories.",
		Instruction: "You are a professional documentation agent utilizing the Open Knowledge Format (OKF). You use connectors (sqlite_connector, mysql_connector, postgresql_connector, bigquery_connector, fs_connector, git_connector) to produce OKF bundles (metadata markdown files) or ingest and sync comments/descriptions from OKF bundles back. Always explain what you are about to do before invoking any tool.",
		Tools:       []tool.Tool{sqliteTool, mysqlTool, postgresqlTool, bigqueryTool, fsTool, gitTool},
	})
}
