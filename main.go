package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rathore/langchain-agent/agent"
	"github.com/rathore/langchain-agent/rag"
	"github.com/rathore/langchain-agent/tools"
)

func main() {
	model := flag.String("model", "llama3.1", "Ollama model to use")
	maxIter := flag.Int("max-iter", 10, "Maximum agent iterations per query")
	wikiPath := flag.String("wiki", "", "Path to Confluence HTML export to index and enable wiki tool")
	qdrantURL := flag.String("qdrant", "http://localhost:6333", "Qdrant server URL")
	indexOnly := flag.Bool("index-only", false, "Only index the wiki, then exit")
	mcpCmd := flag.String("mcp", "", "MCP server command (e.g., 'mcp-filesystem-server /tmp')")
	flag.Parse()

	fmt.Printf("LangChain Agent (model: %s)\n", *model)

	// Initialize tools
	toolList := []tools.Tool{
		&tools.SSHTool{},
		&tools.ShellTool{},
	}

	// MCP tool (only when --mcp is provided)
	if *mcpCmd != "" {
		parts := strings.Fields(*mcpCmd)
		if len(parts) == 0 {
			fmt.Fprintf(os.Stderr, "Invalid --mcp command\n")
			os.Exit(1)
		}
		ctx := context.Background()
		mcpTool, err := tools.NewMCPTool(ctx, parts[0], parts[1:])
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to MCP server: %v\n", err)
			os.Exit(1)
		}
		defer mcpTool.Close()
		toolList = append(toolList, mcpTool)
		fmt.Printf("MCP server connected (%d tools discovered)\n", mcpTool.ToolCount())
	}

	// Handle wiki indexing and tool setup
	if *wikiPath != "" {
		config := rag.DefaultConfig()
		config.WikiPath = *wikiPath
		config.QdrantURL = *qdrantURL

		indexer, err := rag.NewIndexer(config)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to create indexer: %v\n", err)
			os.Exit(1)
		}

		// Index the wiki content
		ctx := context.Background()
		fmt.Printf("Indexing wiki from: %s\n", *wikiPath)
		if err := indexer.Index(ctx); err != nil {
			fmt.Fprintf(os.Stderr, "Failed to index wiki: %v\n", err)
			os.Exit(1)
		}

		if *indexOnly {
			fmt.Println("Indexing complete. Exiting.")
			return
		}

		// Add wiki tool
		wikiTool := tools.NewWikiTool(indexer.GetEmbeddings(), indexer.GetStore())
		toolList = append(toolList, wikiTool)
		fmt.Println("Wiki tool enabled.")
	}

	fmt.Println("Type 'quit' to exit, 'clear' to clear history")
	fmt.Println("---")

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
