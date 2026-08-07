package repository

import (
	"context"
	"testing"
)

type mockFactory struct{}

func (m *mockFactory) Notes() NoteRepository          { return nil }
func (m *mockFactory) Documents() DocumentRepository  { return nil }
func (m *mockFactory) Vectors() VectorRepository      { return nil }
func (m *mockFactory) Graph() GraphRepository        { return nil }
func (m *mockFactory) Settings() SettingsRepository  { return nil }
func (m *mockFactory) Close(ctx context.Context) error { return nil }

func TestRepositoryInterfaces(t *testing.T) {
	var factory RepositoryFactory = &mockFactory{}
	if factory == nil {
		t.Fatal("expected non-nil factory interface")
	}
}
