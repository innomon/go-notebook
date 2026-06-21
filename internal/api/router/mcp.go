package router

import (
	"context"
	"encoding/json"
	"fmt"
	"go-notebook/internal/domain"
	"go-notebook/internal/graphrag"
	"log"
	"net/http"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

// EmptyInput represents an empty JSON schema input for tools
type EmptyInput struct{}

// NotebookInfo represents simplified notebook metadata for MCP clients
type NotebookInfo struct {
	ID          string `json:"id" jsonschema:"The unique record ID of the notebook"`
	Name        string `json:"name" jsonschema:"The name of the notebook"`
	Description string `json:"description" jsonschema:"The description of the notebook"`
}

// ListNotebooksOutput represents the list of notebooks returned to the client
type ListNotebooksOutput struct {
	Notebooks []NotebookInfo `json:"notebooks"`
}

// SearchGraphInput represents the input parameters for querying the GraphRAG pipeline
type SearchGraphInput struct {
	NotebookID string `json:"notebook_id" jsonschema:"The unique record ID of the notebook (e.g. notebook:xyz)"`
	Query      string `json:"query" jsonschema:"The natural language query or question to retrieve context for"`
	Mode       string `json:"mode,omitempty" jsonschema:"The search mode: 'local', 'global', or 'hybrid' (default: 'hybrid')"`
}

// SearchGraphOutput represents the retrieval results from the GraphRAG pipeline
type SearchGraphOutput struct {
	Context  string   `json:"context" jsonschema:"The compiled text context retrieved from the graph"`
	Mode     string   `json:"mode" jsonschema:"The mode of retrieval executed"`
	Sources  []string `json:"sources" jsonschema:"List of source document titles containing relevant facts"`
	Entities []string `json:"entities" jsonschema:"List of entities matched during retrieval"`
}

// GetGraphDataInput represents the parameters for fetching raw graph data
type GetGraphDataInput struct {
	NotebookID string `json:"notebook_id" jsonschema:"The unique record ID of the notebook (e.g. notebook:xyz)"`
	MaxNodes   int    `json:"max_nodes,omitempty" jsonschema:"The maximum number of entities to return (default: 25)"`
}

// GraphDataOutput represents the graph nodes, edges, and communities returned to the client
type GraphDataOutput struct {
	Connections []domain.EntityConnection `json:"connections"`
	Communities []domain.CommunityInfo    `json:"communities"`
	TopNodes    []string                  `json:"top_nodes"`
}

// RegisterMCPRoutes initializes the MCP server and binds it to the ServeMux
func RegisterMCPRoutes(mux *http.ServeMux) {
	// Initialize MCP Server implementation metadata
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "go-notebook-mcp",
		Version: "1.0.0",
	}, nil)

	// 1. Tool: list_notebooks
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "list_notebooks",
			Description: "List all available notebooks with their IDs, names, and descriptions. Useful to determine which notebook ID to query.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input EmptyInput) (*mcp.CallToolResult, ListNotebooksOutput, error) {
			list, err := domain.ListNotebooks(ctx)
			if err != nil {
				return nil, ListNotebooksOutput{}, fmt.Errorf("failed to fetch notebooks: %w", err)
			}

			var notebooks []NotebookInfo
			for _, nb := range list {
				idStr := ""
				if nb.ID != nil {
					idStr = nb.ID.String()
				}
				notebooks = append(notebooks, NotebookInfo{
					ID:          idStr,
					Name:        nb.Name,
					Description: nb.Description,
				})
			}

			jsonData, err := json.MarshalIndent(notebooks, "", "  ")
			if err != nil {
				return nil, ListNotebooksOutput{}, err
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonData)},
				},
			}, ListNotebooksOutput{Notebooks: notebooks}, nil
		},
	)

	// 2. Tool: search_graph
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "search_graph",
			Description: "Search the GraphRAG index of a notebook. Use 'local' mode for entity-specific facts, 'global' mode for high-level thematic questions, and 'hybrid' for combined retrieval.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input SearchGraphInput) (*mcp.CallToolResult, SearchGraphOutput, error) {
			if input.NotebookID == "" {
				return nil, SearchGraphOutput{}, fmt.Errorf("notebook_id is required")
			}
			if input.Query == "" {
				return nil, SearchGraphOutput{}, fmt.Errorf("query is required")
			}
			mode := input.Mode
			if mode == "" {
				mode = "hybrid"
			}

			pipeline, err := graphrag.NewPipeline(ctx)
			if err != nil {
				return nil, SearchGraphOutput{}, fmt.Errorf("failed to initialize GraphRAG pipeline: %w", err)
			}

			res, err := pipeline.Query(ctx, input.NotebookID, input.Query, mode)
			if err != nil {
				return nil, SearchGraphOutput{}, fmt.Errorf("retrieval query failed: %w", err)
			}

			output := SearchGraphOutput{
				Context:  res.Context,
				Mode:     res.Mode,
				Sources:  res.Sources,
				Entities: res.Entities,
			}

			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return nil, SearchGraphOutput{}, err
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonData)},
				},
			}, output, nil
		},
	)

	// 3. Tool: get_graph_data
	mcp.AddTool(server,
		&mcp.Tool{
			Name:        "get_graph_data",
			Description: "Get the raw GraphRAG topology (connections, communities, top nodes) for a notebook. Helpful for mapping relationships.",
		},
		func(ctx context.Context, req *mcp.CallToolRequest, input GetGraphDataInput) (*mcp.CallToolResult, GraphDataOutput, error) {
			if input.NotebookID == "" {
				return nil, GraphDataOutput{}, fmt.Errorf("notebook_id is required")
			}
			maxNodes := input.MaxNodes
			if maxNodes <= 0 {
				maxNodes = 25
			}

			graphData, err := domain.GetNotebookGraphData(ctx, input.NotebookID, maxNodes)
			if err != nil {
				return nil, GraphDataOutput{}, fmt.Errorf("failed to load graph data: %w", err)
			}

			var output GraphDataOutput
			if graphData != nil {
				output.Connections = graphData.Connections
				output.Communities = graphData.Communities
				output.TopNodes = graphData.TopNodes
			}

			jsonData, err := json.MarshalIndent(output, "", "  ")
			if err != nil {
				return nil, GraphDataOutput{}, err
			}

			return &mcp.CallToolResult{
				Content: []mcp.Content{
					&mcp.TextContent{Text: string(jsonData)},
				},
			}, output, nil
		},
	)

	// Create Streamable HTTP SSE handler
	handler := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		return server
	}, nil)

	// Mount on route paths
	log.Println("[API] Registering Model Context Protocol (MCP) Streamable HTTP routes at /api/mcp")
	mux.Handle("GET /api/mcp", handler)
	mux.Handle("POST /api/mcp", handler)
	mux.Handle("GET /api/mcp/", handler)
	mux.Handle("POST /api/mcp/", handler)
}
