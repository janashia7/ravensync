package memory

import (
	"math"
	"path/filepath"
	"testing"
)

func testKey() []byte {
	key := make([]byte, 32)
	copy(key, []byte("test-key-32-bytes-long-padding!!"))
	return key
}

func TestNewStoreAndClose(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath, testKey())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	if store.Count() != 0 {
		t.Fatalf("empty store count = %d, want 0", store.Count())
	}
}

func TestAddAndCount(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath, testKey())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	emb := []float64{0.1, 0.2, 0.3}
	if err := store.Add("user1", "hello world", emb); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if store.Count() != 1 {
		t.Fatalf("count = %d, want 1", store.Count())
	}

	if err := store.Add("user1", "second message", emb); err != nil {
		t.Fatalf("Add: %v", err)
	}
	if store.Count() != 2 {
		t.Fatalf("count = %d, want 2", store.Count())
	}
}

func TestSearchReturnsRelevant(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath, testKey())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	store.Add("user1", "my name is Giorgi", []float64{1.0, 0.0, 0.0})
	store.Add("user1", "I like pizza", []float64{0.0, 1.0, 0.0})
	store.Add("user1", "the weather is nice", []float64{0.0, 0.0, 1.0})

	results := store.Search("user1", []float64{1.0, 0.1, 0.0}, 2)
	if len(results) != 2 {
		t.Fatalf("search returned %d results, want 2", len(results))
	}
	if results[0].Content != "my name is Giorgi" {
		t.Fatalf("top result = %q, want 'my name is Giorgi'", results[0].Content)
	}
}

func TestSearchDifferentUsers(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath, testKey())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	store.Add("alice", "alice data", []float64{1.0, 0.0})
	store.Add("bob", "bob data", []float64{1.0, 0.0})

	results := store.Search("alice", []float64{1.0, 0.0}, 10)
	if len(results) != 1 {
		t.Fatalf("alice results = %d, want 1", len(results))
	}
	if results[0].Content != "alice data" {
		t.Fatalf("result = %q, want 'alice data'", results[0].Content)
	}
}

func TestSearchEmptyStore(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath, testKey())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	results := store.Search("user1", []float64{1.0, 0.0}, 5)
	if results != nil {
		t.Fatalf("expected nil results, got %d", len(results))
	}
}

func TestSearchTopKLimit(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	store, err := NewStore(dbPath, testKey())
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	defer store.Close()

	for i := 0; i < 10; i++ {
		store.Add("user1", "data", []float64{1.0, float64(i)})
	}

	results := store.Search("user1", []float64{1.0, 5.0}, 3)
	if len(results) != 3 {
		t.Fatalf("results = %d, want 3", len(results))
	}
}

func TestCosineSimilarity(t *testing.T) {
	tests := []struct {
		name string
		a, b []float64
		want float64
	}{
		{"identical", []float64{1, 0, 0}, []float64{1, 0, 0}, 1.0},
		{"orthogonal", []float64{1, 0, 0}, []float64{0, 1, 0}, 0.0},
		{"opposite", []float64{1, 0}, []float64{-1, 0}, -1.0},
		{"empty_a", []float64{0, 0}, []float64{1, 0}, 0.0},
		{"different_lengths", []float64{1}, []float64{1, 0}, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cosineSimilarity(tt.a, tt.b)
			if math.Abs(got-tt.want) > 1e-9 {
				t.Fatalf("cosineSimilarity = %f, want %f", got, tt.want)
			}
		})
	}
}

func TestFloat64BytesRoundTrip(t *testing.T) {
	original := []float64{1.5, -2.7, 0.0, math.Pi, math.MaxFloat64}
	b := float64sToBytes(original)
	result := bytesToFloat64s(b)

	if len(result) != len(original) {
		t.Fatalf("length = %d, want %d", len(result), len(original))
	}
	for i := range original {
		if result[i] != original[i] {
			t.Fatalf("index %d: got %f, want %f", i, result[i], original[i])
		}
	}
}

func TestPersistenceAcrossReopen(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	key := testKey()

	store1, err := NewStore(dbPath, key)
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	store1.Add("user1", "persistent data", []float64{1.0, 0.0, 0.0})
	store1.Close()

	store2, err := NewStore(dbPath, key)
	if err != nil {
		t.Fatalf("NewStore reopen: %v", err)
	}
	defer store2.Close()

	if store2.Count() != 1 {
		t.Fatalf("count after reopen = %d, want 1", store2.Count())
	}

	results := store2.Search("user1", []float64{1.0, 0.0, 0.0}, 5)
	if len(results) != 1 || results[0].Content != "persistent data" {
		t.Fatal("data not persisted correctly")
	}
}
