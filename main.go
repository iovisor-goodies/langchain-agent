package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rathore/langchain-agent/agent"
	"github.com/rathore/langchain-agent/tools"
)

func main() {
	model := flag.String("model", "llama3.1", "Ollama model to use")
	maxIter := flag.Int("max-iter", 10, "Maximum agent iterations per query")
	flag.Parse()

	fmt.Printf("LangChain Agent (model: %s)\n", *model)
	fmt.Println("Type 'quit' to exit, 'clear' to clear history")
	fmt.Println("---")

	// Initialize tools
	toolList := []tools.Tool{
		&tools.SSHTool{},
		&tools.MCPTool{},
		&tools.ShellTool{},
	}

	// Create agent
	ag, err := agent.New(agent.Config{
		Model:   *model,
		MaxIter: *maxIter,
		Tools:   toolList,
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create agent: %v\n", err)
		os.Exit(1)
	}

	// REPL loop
	scanner := bufio.NewScanner(os.Stdin)
	ctx := context.Background()

	for {
		fmt.Print("\n> ")
		if !scanner.Scan() {
			break
		}

		input := strings.TrimSpace(scanner.Text())
		if input == "" {
			continue
		}

		switch strings.ToLower(input) {
		case "quit", "exit":
			fmt.Println("Goodbye!")
			return
		case "clear":
			ag.ClearHistory()
			fmt.Println("History cleared.")
			continue
		}

		result, err := ag.Run(ctx, input)
		if err != nil {
			fmt.Printf("\n[Error] %v\n", err)
			continue
		}

		fmt.Printf("\n[Answer]\n%s\n", result)
	}

	if err := scanner.Err(); err != nil {
		fmt.Fprintf(os.Stderr, "Read error: %v\n", err)
	}
}
