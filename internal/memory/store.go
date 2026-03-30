package memory

import (
	"crypto/rand"
	"database/sql"
	"encoding/binary"
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	_ "modernc.org/sqlite"

	"github.com/ravensync/ravensync/internal/crypto"
)

type Memory struct {
	ID        string
	UserID    string
	Content   string
	Embedding []float64
	CreatedAt time.Time
}

type Store struct {
	db    *sql.DB
	key   []byte
	mu    sync.RWMutex
	cache map[string][]cachedMemory
}

type cachedMemory struct {
	ID        string
	Content   string
	Embedding []float64
	CreatedAt time.Time
}

func NewStore(dbPath string, encryptionKey []byte) (*Store, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("set journal mode: %w", err)
	}

	if err := migrate(db); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("migrate database: %w", err)
	}

	s := &Store{
		db:    db,
		key:   encryptionKey,
		cache: make(map[string][]cachedMemory),
	}

	if err := s.loadCache(); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("load memory cache: %w", err)
	}

	return s, nil
}

func migrate(db *sql.DB) error {
	_, err := db.Exec(`
		CREATE TABLE IF NOT EXISTS memories (
			id TEXT PRIMARY KEY,
			user_id TEXT NOT NULL,
			content BLOB NOT NULL,
			embedding BLOB,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		CREATE INDEX IF NOT EXISTS idx_memories_user_id ON memories(user_id);
	`)
	return err
}

func (s *Store) loadCache() error {
	rows, err := s.db.Query("SELECT id, user_id, content, embedding, created_at FROM memories ORDER BY created_at")
	if err != nil {
		return err
	}
	defer func() { _ = rows.Close() }()

	for rows.Next() {
		var id, userID string
		var contentEnc, embeddingEnc []byte
		var createdAt time.Time

		if err := rows.Scan(&id, &userID, &contentEnc, &embeddingEnc, &createdAt); err != nil {
			return err
		}

		content, err := crypto.Decrypt(contentEnc, s.key)
		if err != nil {
			return fmt.Errorf("decrypt memory %s: %w", id, err)
		}

		var embedding []float64
		if embeddingEnc != nil {
			embDec, err := crypto.Decrypt(embeddingEnc, s.key)
			if err != nil {
				return fmt.Errorf("decrypt embedding %s: %w", id, err)
			}
			embedding = bytesToFloat64s(embDec)
		}

		s.cache[userID] = append(s.cache[userID], cachedMemory{
			ID:        id,
			Content:   string(content),
			Embedding: embedding,
			CreatedAt: createdAt,
		})
	}

	return rows.Err()
}

func (s *Store) Add(userID, content string, embedding []float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	id := generateID()

	contentEnc, err := crypto.Encrypt([]byte(content), s.key)
	if err != nil {
		return fmt.Errorf("encrypt content: %w", err)
	}

	var embeddingEnc []byte
	if embedding != nil {
		embBytes := float64sToBytes(embedding)
		embeddingEnc, err = crypto.Encrypt(embBytes, s.key)
		if err != nil {
			return fmt.Errorf("encrypt embedding: %w", err)
		}
	}

	now := time.Now()
	_, err = s.db.Exec(
		"INSERT INTO memories (id, user_id, content, embedding, created_at) VALUES (?, ?, ?, ?, ?)",
		id, userID, contentEnc, embeddingEnc, now,
	)
	if err != nil {
		return fmt.Errorf("insert memory: %w", err)
	}

	s.cache[userID] = append(s.cache[userID], cachedMemory{
		ID:        id,
		Content:   content,
		Embedding: embedding,
		CreatedAt: now,
	})

	return nil
}

func (s *Store) Search(userID string, queryEmbedding []float64, topK int) []Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	memories := s.cache[userID]
	if len(memories) == 0 {
		return nil
	}

	type scored struct {
		mem   cachedMemory
		score float64
	}

	var results []scored
	for _, m := range memories {
		if m.Embedding == nil {
			continue
		}
		score := cosineSimilarity(queryEmbedding, m.Embedding)
		results = append(results, scored{mem: m, score: score})
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].score > results[j].score
	})

	if topK > len(results) {
		topK = len(results)
	}

	out := make([]Memory, topK)
	for i, r := range results[:topK] {
		out[i] = Memory{
			ID:        r.mem.ID,
			UserID:    userID,
			Content:   r.mem.Content,
			Embedding: r.mem.Embedding,
			CreatedAt: r.mem.CreatedAt,
		}
	}

	return out
}

func (s *Store) Count() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	total := 0
	for _, mems := range s.cache {
		total += len(mems)
	}
	return total
}

func (s *Store) CountUser(userID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.cache[userID])
}

func (s *Store) ListRecent(userID string, n int) []Memory {
	s.mu.RLock()
	defer s.mu.RUnlock()

	mems := s.cache[userID]
	if len(mems) == 0 {
		return nil
	}

	start := len(mems) - n
	if start < 0 {
		start = 0
	}

	out := make([]Memory, 0, n)
	for _, m := range mems[start:] {
		out = append(out, Memory{
			ID:        m.ID,
			UserID:    userID,
			Content:   m.Content,
			CreatedAt: m.CreatedAt,
		})
	}
	return out
}

func (s *Store) DeleteUser(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec("DELETE FROM memories WHERE user_id = ?", userID)
	if err != nil {
		return fmt.Errorf("delete user memories: %w", err)
	}
	delete(s.cache, userID)
	return nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x", b)
}

func cosineSimilarity(a, b []float64) float64 {
	if len(a) != len(b) {
		return 0
	}
	var dot, normA, normB float64
	for i := range a {
		dot += a[i] * b[i]
		normA += a[i] * a[i]
		normB += b[i] * b[i]
	}
	if normA == 0 || normB == 0 {
		return 0
	}
	return dot / (math.Sqrt(normA) * math.Sqrt(normB))
}

func float64sToBytes(fs []float64) []byte {
	buf := make([]byte, len(fs)*8)
	for i, f := range fs {
		binary.LittleEndian.PutUint64(buf[i*8:], math.Float64bits(f))
	}
	return buf
}

func bytesToFloat64s(b []byte) []float64 {
	fs := make([]float64, len(b)/8)
	for i := range fs {
		fs[i] = math.Float64frombits(binary.LittleEndian.Uint64(b[i*8:]))
	}
	return fs
}
