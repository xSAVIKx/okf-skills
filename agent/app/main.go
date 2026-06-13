// Package main implements the OKF (Open Knowledge Format) Agent.
// It uses the Google Agent Development Kit (ADK) for Go to initialize an LLM agent
// that wraps our database, filesystem, and git skills as tools.
package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"
	"strings"

	"google.golang.org/adk/agent"
	"google.golang.org/adk/model/gemini"
	"google.golang.org/adk/runner"
	"google.golang.org/adk/session"
	"google.golang.org/genai"
)

// main is the okf-agent entrypoint. It initializes Gemini, configures tools,
// creates the session storage service, and spins up a local command-line chat session loop.
func main() {
	ctx := context.Background()
	apiKey := os.Getenv("GEMINI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("GOOGLE_API_KEY")
	}
	if apiKey == "" {
		log.Fatal("GEMINI_API_KEY or GOOGLE_API_KEY environment variable is required")
	}

	// Initialize the Gemini 2.5 Flash model via Go GenAI SDK
	model, err := gemini.NewModel(ctx, "gemini-2.5-flash", &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		log.Fatalf("Failed to initialize Gemini model: %v", err)
	}

	// Build the core agent with modular tools loaded
	a, err := BuildAgent(model)
	if err != nil {
		log.Fatalf("Failed to build agent: %v", err)
	}

	// Initialize session storage service (In-Memory implementation)
	sessSvc := session.InMemoryService()
	r, err := runner.New(runner.Config{
		AppName:        "okf-agent",
		Agent:          a,
		SessionService: sessSvc,
	})
	if err != nil {
		log.Fatalf("Failed to create runner: %v", err)
	}

	fmt.Println("=================================================================")
	fmt.Println("   Open Knowledge Format (OKF) Agent (Go ADK)")
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

		// Ensure session exists in storage before running the session
		_, err = sessSvc.Get(ctx, &session.GetRequest{
			AppName:   "okf-agent",
			UserID:    userID,
			SessionID: sessionID,
		})
		if err != nil {
			_, _ = sessSvc.Create(ctx, &session.CreateRequest{
				AppName:   "okf-agent",
				UserID:    userID,
				SessionID: sessionID,
			})
		}

		// Run session and stream response events
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
