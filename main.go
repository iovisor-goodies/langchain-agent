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

// stringSlice implements flag.Value for repeatable string flags.
type stringSlice []string

func (s *stringSlice) String() string { return strings.Join(*s, ", ") }
func (s *stringSlice) Set(val string) error {
	*s = append(*s, val)
	return nil
}

// parseMCPSpec parses an MCP spec into a tool name and target command/URL.
// Format: [label:]command-or-url
// If label is provided: tool name is "mcp_<label>"
// If no label: "mcp" for index 0, "mcp2" for index 1, etc.
func parseMCPSpec(spec string, index int) (name, target string) {
	// Check for label:target format.
	// Only split if the part before ':' doesn't look like a URL scheme.
	if i := strings.Index(spec, ":"); i > 0 {
		prefix := spec[:i]
		if prefix != "http" && prefix != "https" {
			label := prefix
			target = strings.TrimSpace(spec[i+1:])
			return "mcp_" + label, target
		}
	}

	// No label — auto-generate name
	if index == 0 {
		return "mcp", spec
	}
	return fmt.Sprintf("mcp%d", index+1), spec
}

func main() {
	model := flag.String("model", "llama3.1", "Ollama model to use")
	maxIter := flag.Int("max-iter", 10, "Maximum agent iterations per query")
	wikiPath := flag.String("wiki", "", "Path to Confluence HTML export to index and enable wiki tool")
	qdrantURL := flag.String("qdrant", "http://localhost:6333", "Qdrant server URL")
	indexOnly := flag.Bool("index-only", false, "Only index the wiki, then exit")
	var mcpSpecs stringSlice
	flag.Var(&mcpSpecs, "mcp", "MCP server (repeatable). Format: [label:]command-or-url")
	flag.Parse()

	fmt.Printf("LangChain Agent (model: %s)\n", *model)

	// Initialize tools
	toolList := []tools.Tool{
		&tools.SSHTool{},
		&tools.ShellTool{},
	}

	// MCP tools (only when --mcp is provided)
	for i, spec := range mcpSpecs {
		name, target := parseMCPSpec(spec, i)
		ctx := context.Background()
		var mcpTool *tools.MCPTool
		var err error

		if strings.HasPrefix(target, "http://") || strings.HasPrefix(target, "https://") {
			mcpTool, err = tools.NewMCPToolFromURL(ctx, name, target)
		} else {
			parts := strings.Fields(target)
			if len(parts) == 0 {
				fmt.Fprintf(os.Stderr, "Invalid --mcp command: %s\n", spec)
				os.Exit(1)
			}
			mcpTool, err = tools.NewMCPTool(ctx, name, parts[0], parts[1:])
		}
		if err != nil {
			fmt.Fprintf(os.Stderr, "Failed to connect to MCP server %q: %v\n", name, err)
			os.Exit(1)
		}
		defer mcpTool.Close()
		toolList = append(toolList, mcpTool)
		fmt.Printf("MCP server %q connected (%d tools discovered)\n", name, mcpTool.ToolCount())
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

	fmt.Println("Type /help for commands")
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
		case "quit", "exit", "/exit":
			fmt.Println("Goodbye!")
			return
		case "clear", "/clear":
			ag.ClearHistory()
			fmt.Println("History cleared.")
			continue
		case "/help":
			fmt.Println("Commands:")
			fmt.Println("  /help   - Show this help message")
			fmt.Println("  /clear  - Clear conversation history")
			fmt.Println("  /exit   - Exit the agent")
			fmt.Println("")
			fmt.Println("Anything else is sent to the LLM as a prompt.")
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
