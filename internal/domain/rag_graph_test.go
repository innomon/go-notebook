package domain

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/surrealdb/surrealdb.go/pkg/models"
)

func TestRAGEntitySerialization(t *testing.T) {
	nbID := models.NewRecordID("notebook", "proj123")
	srcID1 := models.NewRecordID("source", "docA")
	srcID2 := models.NewRecordID("source", "docB")
	entID := models.NewRecordID("RAGEntity", "entityX")

	entity := RAGEntity{
		ID:       &entID,
		Name:     "artificial intelligence",
		Count:    2,
		Sources:  []*models.RecordID{&srcID1, &srcID2},
		Notebook: &nbID,
		Created:  time.Now(),
	}

	// Test Marshal JSON
	data, err := json.Marshal(entity)
	if err != nil {
		t.Fatalf("failed to marshal RAGEntity: %v", err)
	}

	jsonStr := string(data)
	if !strings.Contains(jsonStr, `"sources"`) {
		t.Errorf("expected json string to contain sources field, got: %s", jsonStr)
	}
	if !strings.Contains(jsonStr, `"Table":"source"`) || !strings.Contains(jsonStr, `"ID":"docA"`) {
		t.Errorf("expected json string to contain source Table and ID representation, got: %s", jsonStr)
	}

	// Test Unmarshal JSON
	var unmarshaled RAGEntity
	err = json.Unmarshal(data, &unmarshaled)
	if err != nil {
		t.Fatalf("failed to unmarshal RAGEntity: %v", err)
	}

	if unmarshaled.Name != entity.Name {
		t.Errorf("expected name %q, got %q", entity.Name, unmarshaled.Name)
	}

	if len(unmarshaled.Sources) != 2 {
		t.Errorf("expected 2 sources, got %d", len(unmarshaled.Sources))
	} else {
		if unmarshaled.Sources[0].String() != srcID1.String() {
			t.Errorf("expected first source to be %s, got %s", srcID1, unmarshaled.Sources[0])
		}
	}
}
