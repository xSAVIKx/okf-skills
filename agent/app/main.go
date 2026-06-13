package main

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/agent/llmagent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/adk/tool"
	"google.golang.org/adk/tool/functiontool"
	"google.golang.org/genai"
)

// SQLite connection arguments
type SqliteArgs struct {
	Action    string `json:"action" description:"The action to perform, either 'produce' (extract schema to OKF) or 'ingest' (validate/sync schema from OKF)."`
	DbPath    string `json:"db_path" description:"The relative or absolute path to the SQLite database file."`
	OutDir    string `json:"out_dir,omitempty" description:"The output directory where the OKF bundle will be created (required for produce)."`
	BundleDir string `json:"bundle_dir,omitempty" description:"The directory of the existing OKF bundle (required for ingest)."`
	Sync      bool   `json:"sync,omitempty" description:"If true, synchronizes the database schema (optional for ingest)."`
}

type SqliteResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// MySQL connection arguments
type MysqlArgs struct {
	Action    string `json:"action" description:"The action to perform, either 'produce' (extract schema to OKF) or 'ingest' (sync comments from OKF)."`
	Host      string `json:"host,omitempty" description:"The MySQL host (default: localhost)."`
	Port      int    `json:"port,omitempty" description:"The MySQL port (default: 3306)."`
	User      string `json:"user" description:"The database username."`
	Password  string `json:"password" description:"The database password."`
	Db        string `json:"db" description:"The target database/schema name."`
	OutDir    string `json:"out_dir,omitempty" description:"The output directory where the OKF bundle will be created (required for produce)."`
	BundleDir string `json:"bundle_dir,omitempty" description:"The directory of the existing OKF bundle (required for ingest)."`
	Sync      bool   `json:"sync,omitempty" description:"If true, executes DDL statements to synchronize descriptions (optional for ingest)."`
}

type MysqlResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// PostgreSQL connection arguments
type PostgresqlArgs struct {
	Action    string `json:"action" description:"The action to perform, either 'produce' (extract schema to OKF) or 'ingest' (sync comments from OKF)."`
	Host      string `json:"host,omitempty" description:"The PostgreSQL host (default: localhost)."`
	Port      int    `json:"port,omitempty" description:"The PostgreSQL port (default: 5432)."`
	User      string `json:"user" description:"The database username (default: postgres)."`
	Password  string `json:"password" description:"The database password."`
	Db        string `json:"db" description:"The target database name."`
	Schema    string `json:"schema,omitempty" description:"The target schema (default: public)."`
	OutDir    string `json:"out_dir,omitempty" description:"The output directory where the OKF bundle will be created (required for produce)."`
	BundleDir string `json:"bundle_dir,omitempty" description:"The directory of the existing OKF bundle (required for ingest)."`
	Sync      bool   `json:"sync,omitempty" description:"If true, executes COMMENT ON statements to synchronize comments (optional for ingest)."`
}

type PostgresqlResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

// BigQuery connection arguments
type BigqueryArgs struct {
	Action    string `json:"action" description:"The action to perform, either 'produce' (extract schema to OKF) or 'ingest' (sync descriptions from OKF)."`
	Project   string `json:"project" description:"The Google Cloud Project ID."`
	Dataset   string `json:"dataset" description:"The BigQuery Dataset ID."`
	OutDir    string `json:"out_dir,omitempty" description:"The output directory where the OKF bundle will be created (required for produce)."`
	BundleDir string `json:"bundle_dir,omitempty" description:"The directory of the existing OKF bundle (required for ingest)."`
	Sync      bool   `json:"sync,omitempty" description:"If true, calls BigQuery API to update schema descriptions (optional for ingest)."`
}

type BigqueryResult struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

func runSqliteTool(ctx tool.Context, args SqliteArgs) (SqliteResult, error) {
	binaryName := "okf-sqlite"
	if runtime.GOOS == "windows" {
		binaryName = "okf-sqlite.exe"
	}
	binaryPath := filepath.Join("..", "..", "skills", "okf-sqlite", binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		binaryPath = filepath.Join("skills", "okf-sqlite", binaryName)
	}

	cmdArgs := []string{args.Action, "--db", args.DbPath}
	if args.Action == "produce" {
		cmdArgs = append(cmdArgs, "--out", args.OutDir)
	} else {
		cmdArgs = append(cmdArgs, "--bundle", args.BundleDir)
		if args.Sync {
			cmdArgs = append(cmdArgs, "--sync")
		}
	}

	cmd := exec.Command(binaryPath, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return SqliteResult{Success: false, Message: fmt.Sprintf("Error running sqlite tool: %v. Stderr: %s", err, stderr.String())}, nil
	}
	return SqliteResult{Success: true, Message: stdout.String()}, nil
}

func runMysqlTool(ctx tool.Context, args MysqlArgs) (MysqlResult, error) {
	binaryName := "okf-mysql"
	if runtime.GOOS == "windows" {
		binaryName = "okf-mysql.exe"
	}
	binaryPath := filepath.Join("..", "..", "skills", "okf-mysql", binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		binaryPath = filepath.Join("skills", "okf-mysql", binaryName)
	}

	hostVal := "localhost"
	if args.Host != "" {
		hostVal = args.Host
	}
	portVal := 3306
	if args.Port != 0 {
		portVal = args.Port
	}

	cmdArgs := []string{
		args.Action,
		"--host", hostVal,
		"--port", fmt.Sprintf("%d", portVal),
		"--user", args.User,
		"--password", args.Password,
		"--db", args.Db,
	}

	if args.Action == "produce" {
		cmdArgs = append(cmdArgs, "--out", args.OutDir)
	} else {
		cmdArgs = append(cmdArgs, "--bundle", args.BundleDir)
		if args.Sync {
			cmdArgs = append(cmdArgs, "--sync")
		}
	}

	cmd := exec.Command(binaryPath, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return MysqlResult{Success: false, Message: fmt.Sprintf("Error running mysql tool: %v. Stderr: %s", err, stderr.String())}, nil
	}
	return MysqlResult{Success: true, Message: stdout.String()}, nil
}

func runPostgresqlTool(ctx tool.Context, args PostgresqlArgs) (PostgresqlResult, error) {
	binaryName := "okf-postgresql"
	if runtime.GOOS == "windows" {
		binaryName = "okf-postgresql.exe"
	}
	binaryPath := filepath.Join("..", "..", "skills", "okf-postgresql", binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		binaryPath = filepath.Join("skills", "okf-postgresql", binaryName)
	}

	hostVal := "localhost"
	if args.Host != "" {
		hostVal = args.Host
	}
	portVal := 5432
	if args.Port != 0 {
		portVal = args.Port
	}
	schemaVal := "public"
	if args.Schema != "" {
		schemaVal = args.Schema
	}

	cmdArgs := []string{
		args.Action,
		"--host", hostVal,
		"--port", fmt.Sprintf("%d", portVal),
		"--user", args.User,
		"--password", args.Password,
		"--db", args.Db,
		"--schema", schemaVal,
	}

	if args.Action == "produce" {
		cmdArgs = append(cmdArgs, "--out", args.OutDir)
	} else {
		cmdArgs = append(cmdArgs, "--bundle", args.BundleDir)
		if args.Sync {
			cmdArgs = append(cmdArgs, "--sync")
		}
	}

	cmd := exec.Command(binaryPath, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return PostgresqlResult{Success: false, Message: fmt.Sprintf("Error running postgresql tool: %v. Stderr: %s", err, stderr.String())}, nil
	}
	return PostgresqlResult{Success: true, Message: stdout.String()}, nil
}

func runBigqueryTool(ctx tool.Context, args BigqueryArgs) (BigqueryResult, error) {
	binaryName := "okf-bigquery"
	if runtime.GOOS == "windows" {
		binaryName = "okf-bigquery.exe"
	}
	binaryPath := filepath.Join("..", "..", "skills", "okf-bigquery", binaryName)
	if _, err := os.Stat(binaryPath); os.IsNotExist(err) {
		binaryPath = filepath.Join("skills", "okf-bigquery", binaryName)
	}

	cmdArgs := []string{
		args.Action,
		"--project", args.Project,
		"--dataset", args.Dataset,
	}

	if args.Action == "produce" {
		cmdArgs = append(cmdArgs, "--out", args.OutDir)
	} else {
		cmdArgs = append(cmdArgs, "--bundle", args.BundleDir)
		if args.Sync {
			cmdArgs = append(cmdArgs, "--sync")
		}
	}

	cmd := exec.Command(binaryPath, cmdArgs...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return BigqueryResult{Success: false, Message: fmt.Sprintf("Error running bigquery tool: %v. Stderr: %s", err, stderr.String())}, nil
	}
	return BigqueryResult{Success: true, Message: stdout.String()}, nil
}

func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY or GOOGLE_API_KEY environment variable is required")
	}

	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to initialize Gemini model: %v", err)
	}

	// Create ADK tools
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

	// Create ADK agent
	a, err := llmagent.New(llmagent.Config{
		Name:        "okf-reference-agent",
		Model:       model,
		Description: "An AI agent that creates and ingests Open Knowledge Format (OKF) bundles for databases.",
		Instruction: "You are a professional database documentation agent utilizing the Open Knowledge Format (OKF). You use database connectors (sqlite_connector, mysql_connector, postgresql_connector, bigquery_connector) to produce OKF bundles (metadata markdown files) from databases or ingest and sync comments/descriptions from OKF bundles back into databases. Always explain what you are about to do before invoking any tool.",
		Tools:       []tool.Tool{sqliteTool, mysqlTool, postgresqlTool, bigqueryTool},
	})
	if err != nil {
		log.Fatalf("Failed to create agent: %v", err)
	}

	// Create ADK runner
	sessSvc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:        "okf-reference-agent",
		Agent:          a,
		SessionService: sessSvc,
	})
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	fmt.Println("=================================================================")
	fmt.Println("   Open Knowledge Format (OKF) Reference Agent (Go ADK)")
	fmt.Println("=================================================================")
	fmt.Println("Enter your instructions (e.g. 'Generate an OKF bundle from sqlite database test.db to ./bundle'):")
	fmt.Println("Press Ctrl+C to exit.")
	fmt.Println()

	scanner := bufio.NewScanner(os.Stdin)
	userID := "default-user"
	sessionID := "default-session"

	for {
		fmt.Print("User> ")
		if !scanner.Scan() {
			break
		}
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}

		userMsg := &genai.Content{
			Role: "user",
			Parts: []*genai.Part{
				{Text: text},
			},
		}

		// Ensure session exists in storage
		_, err = sessSvc.Get(ctx, &session.GetRequest{
			AppName:   "okf-reference-agent",
			UserID:    userID,
			SessionID: sessionID,
		})
		if err != nil {
			_, _ = sessSvc.Create(ctx, &session.CreateRequest{
				AppName:   "okf-reference-agent",
				UserID:    userID,
				SessionID: sessionID,
			})
		}

		// Run session
		for event, err := range r.Run(ctx, userID, sessionID, userMsg, agent.RunConfig{}) {
			if err != nil {
				fmt.Printf("\nError encountered: %v\n", err)
				break
			}
			if event.Content != nil {
				for _, part := range event.Content.Parts {
					if part.Text != "" {
						fmt.Print(part.Text)
					}
				}
			}
		}
		fmt.Println()
	}
}
