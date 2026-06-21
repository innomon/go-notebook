package graphrag

import (
	"context"
	"crypto/sha256"
	"fmt"
	"go-notebook/internal/ai"
	"go-notebook/internal/db"
	"go-notebook/internal/domain"
	"os"
	"strings"
	"testing"
	"time"
)

type MockAIClient struct {
	GenerateTextFn func(ctx context.Context, systemPrompt, userPrompt string) (string, error)
	AnalyzeImageFn  func(ctx context.Context, filePath string, prompt string) (string, error)
}

func (m *MockAIClient) EmbedText(ctx context.Context, text string) ([]float32, error) {
	return make([]float32, 1536), nil
}

func (m *MockAIClient) GenerateChatStream(ctx context.Context, systemPrompt string, messages []ai.ChatMessage, onToken func(string)) error {
	return nil
}

func (m *MockAIClient) GenerateText(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if m.GenerateTextFn != nil {
		return m.GenerateTextFn(ctx, systemPrompt, userPrompt)
	}
	return `{"entities":[], "relationships":[]}`, nil
}

func (m *MockAIClient) GenerateSpeech(ctx context.Context, text string, voice string) ([]byte, error) {
	return nil, nil
}

func (m *MockAIClient) TranscribeAudio(ctx context.Context, filePath string) (string, error) {
	return "", nil
}

func (m *MockAIClient) AnalyzeImage(ctx context.Context, filePath string, prompt string) (string, error) {
	if m.AnalyzeImageFn != nil {
		return m.AnalyzeImageFn(ctx, filePath, prompt)
	}
	return "", nil
}

func TestIncrementalPipeline(t *testing.T) {
	os.Setenv("SURREAL_URL", "ws://localhost:8000/rpc")
	os.Setenv("SURREAL_USER", "root")
	os.Setenv("SURREAL_PASSWORD", "root")
	os.Setenv("SURREAL_NAMESPACE", "open_notebook_test")
	os.Setenv("SURREAL_DATABASE", "open_notebook_test")

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := db.Init(ctx); err != nil {
		t.Skip("SurrealDB offline")
	}
	defer db.Close(ctx)

	// Clean test database and run migrations
	_, _ = db.RepoQuery[any](ctx, "REMOVE DATABASE open_notebook_test;", nil)
	_, _ = db.RepoQuery[any](ctx, "DEFINE DATABASE open_notebook_test;", nil)
	_ = db.RunMigrationUp(ctx)

	notebookID := "nb_pipeline_test"
	notebookRecordID := db.EnsureRecordID("notebook", notebookID)
	// Create the notebook record
	_, err := db.RepoCreateWithID[any](ctx, "notebook", notebookID, map[string]any{
		"name": "Pipeline Test Notebook",
	})
	if err != nil {
		t.Fatalf("failed to create notebook: %v", err)
	}

	// 1. Create two sources
	source1ID := "src_p1"
	source2ID := "src_p2"

	text1 := "Artificial Intelligence is evolving rapidly."
	text2 := "Machine Learning is a subset of Artificial Intelligence."

	hashSum1 := sha256.Sum256([]byte(text1))
	hash1 := fmt.Sprintf("%x", hashSum1)

	hashSum2 := sha256.Sum256([]byte(text2))
	hash2 := fmt.Sprintf("%x", hashSum2)

	src1, err := db.RepoCreateWithID[domain.Source](ctx, "source", source1ID, map[string]any{
		"title":     "AI Doc",
		"full_text": text1,
		"hash":      hash1,
	})
	if err != nil {
		t.Fatalf("failed to create source 1: %v", err)
	}

	src2, err := db.RepoCreateWithID[domain.Source](ctx, "source", source2ID, map[string]any{
		"title":     "ML Doc",
		"full_text": text2,
		"hash":      hash2,
	})
	if err != nil {
		t.Fatalf("failed to create source 2: %v", err)
	}

	// Link sources to notebook
	_ = domain.LinkSourceToNotebook(ctx, src1.ID.String(), notebookID)
	_ = domain.LinkSourceToNotebook(ctx, src2.ID.String(), notebookID)

	// Create Mock AI Clients
	extractedSourceCalled := make(map[string]int)
	mockChat := &MockAIClient{
		GenerateTextFn: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			if strings.Contains(systemPrompt, "extracting key named entities") {
				if strings.Contains(userPrompt, "Artificial Intelligence") && strings.Contains(userPrompt, "evolving") {
					extractedSourceCalled["src_p1"]++
					return `{"entities":[{"name":"artificial intelligence","type":"CONCEPT"},{"name":"evolving","type":"CONCEPT"}],"relationships":[{"source":"artificial intelligence","target":"evolving"}]}`, nil
				}
				if strings.Contains(userPrompt, "Machine Learning") {
					extractedSourceCalled["src_p2"]++
					return `{"entities":[{"name":"artificial intelligence","type":"CONCEPT"},{"name":"machine learning","type":"CONCEPT"}],"relationships":[{"source":"machine learning","target":"artificial intelligence"}]}`, nil
				}
			}
			// Fallback for community summary generation
			if strings.Contains(systemPrompt, "community's report") || strings.Contains(systemPrompt, "summarize") {
				return "This is a summary of the community.", nil
			}
			return `{"entities":[], "relationships":[]}`, nil
		},
	}

	pipeline := &Pipeline{
		chatClient:  mockChat,
		embedClient: mockChat,
	}

	// First build: both sources should be processed
	err = pipeline.BuildGraph(ctx, notebookID)
	if err != nil {
		t.Fatalf("BuildGraph failed: %v", err)
	}

	if extractedSourceCalled["src_p1"] != 1 || extractedSourceCalled["src_p2"] != 1 {
		t.Errorf("expected both sources to be extracted once, got src_p1: %d, src_p2: %d", extractedSourceCalled["src_p1"], extractedSourceCalled["src_p2"])
	}

	// Verify database state: "artificial intelligence" should have count 2, and both sources in the array
	res, err := db.RepoQuery[[]domain.RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = 'artificial intelligence';", map[string]any{"nb": notebookRecordID})
	if err != nil || len(*res) == 0 {
		t.Fatalf("failed to retrieve entity 'artificial intelligence': %v", err)
	}
	ent := (*res)[0]
	if ent.Count != 2 || len(ent.Sources) != 2 {
		t.Errorf("expected entity count 2, sources len 2, got count %d, sources %v", ent.Count, ent.Sources)
	}

	// Second build (nothing changed): sources should be skipped
	err = pipeline.BuildGraph(ctx, notebookID)
	if err != nil {
		t.Fatalf("BuildGraph failed on second run: %v", err)
	}

	if extractedSourceCalled["src_p1"] != 1 || extractedSourceCalled["src_p2"] != 1 {
		t.Errorf("expected no additional extraction calls, got src_p1: %d, src_p2: %d", extractedSourceCalled["src_p1"], extractedSourceCalled["src_p2"])
	}

	// Third build (source 1 modified): only source 1 should be re-extracted
	text1Mod := "Artificial Intelligence is super powerful."
	hashSum1Mod := sha256.Sum256([]byte(text1Mod))
	hash1Mod := fmt.Sprintf("%x", hashSum1Mod)

	_, err = db.RepoUpdate[any](ctx, "source", src1.ID.String(), map[string]any{
		"full_text": text1Mod,
		"hash":      hash1Mod,
	})
	if err != nil {
		t.Fatalf("failed to update source 1: %v", err)
	}

	mockChat.GenerateTextFn = func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
		if strings.Contains(systemPrompt, "extracting key named entities") {
			if strings.Contains(userPrompt, "super powerful") {
				extractedSourceCalled["src_p1"]++
				return `{"entities":[{"name":"artificial intelligence","type":"CONCEPT"},{"name":"super powerful","type":"CONCEPT"}],"relationships":[{"source":"artificial intelligence","target":"super powerful"}]}`, nil
			}
			if strings.Contains(userPrompt, "Machine Learning") {
				extractedSourceCalled["src_p2"]++
				return `{"entities":[{"name":"artificial intelligence","type":"CONCEPT"},{"name":"machine learning","type":"CONCEPT"}],"relationships":[{"source":"machine learning","target":"artificial intelligence"}]}`, nil
			}
		}
		// Fallback for community summary generation
		if strings.Contains(systemPrompt, "community's report") || strings.Contains(systemPrompt, "summarize") {
			return "This is a summary of the community.", nil
		}
		return `{"entities":[], "relationships":[]}`, nil
	}

	err = pipeline.BuildGraph(ctx, notebookID)
	if err != nil {
		t.Fatalf("BuildGraph failed on third run: %v", err)
	}

	if extractedSourceCalled["src_p1"] != 2 {
		t.Errorf("expected src_p1 to be extracted a second time, count: %d", extractedSourceCalled["src_p1"])
	}
	if extractedSourceCalled["src_p2"] != 1 {
		t.Errorf("expected src_p2 to remain at 1 extraction call, count: %d", extractedSourceCalled["src_p2"])
	}

	// Verify old lineage "evolving" is deleted (since it had only src_p1, which was modified/cleared)
	resEvolving, err := db.RepoQuery[[]domain.RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = 'evolving';", map[string]any{"nb": notebookRecordID})
	if err != nil {
		t.Fatalf("query for evolving failed: %v", err)
	}
	if len(*resEvolving) != 0 {
		t.Errorf("expected entity 'evolving' to be deleted, but found: %v", *resEvolving)
	}

	// Verify new entity "super powerful" exists and has count 1
	resPowerful, err := db.RepoQuery[[]domain.RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = 'super powerful';", map[string]any{"nb": notebookRecordID})
	if err != nil || len(*resPowerful) == 0 {
		t.Fatalf("failed to retrieve new entity 'super powerful': %v", err)
	}
	if (*resPowerful)[0].Count != 1 {
		t.Errorf("expected 'super powerful' count 1, got: %d", (*resPowerful)[0].Count)
	}

	// 4. Delete/Unlink source 2 and build graph again
	_, err = db.RepoQuery[any](ctx, "DELETE reference WHERE in = $src AND out = $nb;", map[string]any{"src": db.EnsureRecordID("source", source2ID), "nb": notebookRecordID})
	if err != nil {
		t.Fatalf("failed to unlink source 2: %v", err)
	}

	err = pipeline.BuildGraph(ctx, notebookID)
	if err != nil {
		t.Fatalf("BuildGraph failed on fourth run (after delete): %v", err)
	}

	// Machine learning entity should be gone because its only source was unlinked
	resML, err := db.RepoQuery[[]domain.RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = 'machine learning';", map[string]any{"nb": notebookRecordID})
	if err != nil {
		t.Fatalf("query for machine learning failed: %v", err)
	}
	if len(*resML) != 0 {
		t.Errorf("expected entity 'machine learning' to be deleted, but found: %v", *resML)
	}

	// Artificial intelligence entity should remain (since src_p1 is still linked) but count should be 1
	resAI, err := db.RepoQuery[[]domain.RAGEntity](ctx, "SELECT * FROM RAGEntity WHERE notebook = $nb AND name = 'artificial intelligence';", map[string]any{"nb": notebookRecordID})
	if err != nil || len(*resAI) == 0 {
		t.Fatalf("artificial intelligence should still exist, but not found: %v", err)
	}
	if (*resAI)[0].Count != 1 {
		t.Errorf("expected artificial intelligence count to be 1, got %d", (*resAI)[0].Count)
	}
}
