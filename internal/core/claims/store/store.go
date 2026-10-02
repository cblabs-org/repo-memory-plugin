package store

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/cblabs-org/repo-memory-plugin/internal/core/claims"
)

var (
	ErrNotFound = errors.New("claim not found")
)

// InverseRelations represents the inverse lookup index for a claim.
type InverseRelations struct {
	SupportedBy    []string
	ContradictedBy []string
	SupersededBy   []string
	ExplainedBy    []string
	RefinedBy      []string
	DependedOnBy   []string
}

type Store struct {
	baseDir string

	mu sync.RWMutex
	// claims maps claim ID to its file path
	claims map[string]string

	// relationIndex maps targetClaimID -> relation type -> list of source claim IDs
	relationIndex map[string]map[string][]string
}

func New(baseDir string) (*Store, error) {
	s := &Store{
		baseDir:       baseDir,
		claims:        make(map[string]string),
		relationIndex: make(map[string]map[string][]string),
	}
	if err := s.loadAll(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) loadAll() error {
	if _, err := os.Stat(s.baseDir); os.IsNotExist(err) {
		return os.MkdirAll(s.baseDir, 0755)
	}

	return filepath.WalkDir(s.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			data, err := os.ReadFile(path)
			if err != nil {
				return err
			}
			claim, err := claims.Parse(data)
			if err != nil {
				// Skip or just return err. Let's return err for strictness.
				return err
			}
			s.claims[claim.ID] = path
			s.indexRelationsUnsafe(claim)
		}
		return nil
	})
}

func (s *Store) indexRelationsUnsafe(claim *claims.Claim) {
	if claim.Relations == nil {
		return
	}
	s.addRelation(claim.ID, "supports", claim.Relations.Supports)
	s.addRelation(claim.ID, "contradicts", claim.Relations.Contradicts)
	s.addRelation(claim.ID, "supersedes", claim.Relations.Supersedes)
	s.addRelation(claim.ID, "explains", claim.Relations.Explains)
	s.addRelation(claim.ID, "refines", claim.Relations.Refines)
	s.addRelation(claim.ID, "depends_on", claim.Relations.DependsOn)
}

func (s *Store) addRelation(sourceID string, relType string, targets []string) {
	for _, target := range targets {
		if s.relationIndex[target] == nil {
			s.relationIndex[target] = make(map[string][]string)
		}
		s.relationIndex[target][relType] = append(s.relationIndex[target][relType], sourceID)
	}
}

func (s *Store) removeRelationsUnsafe(claim *claims.Claim) {
	if claim.Relations == nil {
		return
	}
	s.removeRelation(claim.ID, "supports", claim.Relations.Supports)
	s.removeRelation(claim.ID, "contradicts", claim.Relations.Contradicts)
	s.removeRelation(claim.ID, "supersedes", claim.Relations.Supersedes)
	s.removeRelation(claim.ID, "explains", claim.Relations.Explains)
	s.removeRelation(claim.ID, "refines", claim.Relations.Refines)
	s.removeRelation(claim.ID, "depends_on", claim.Relations.DependsOn)
}

func (s *Store) removeRelation(sourceID string, relType string, targets []string) {
	for _, target := range targets {
		if s.relationIndex[target] == nil {
			continue
		}
		var updated []string
		for _, src := range s.relationIndex[target][relType] {
			if src != sourceID {
				updated = append(updated, src)
			}
		}
		if len(updated) == 0 {
			delete(s.relationIndex[target], relType)
			if len(s.relationIndex[target]) == 0 {
				delete(s.relationIndex, target)
			}
		} else {
			s.relationIndex[target][relType] = updated
		}
	}
}

func (s *Store) getClaimPath(claim *claims.Claim) string {
	created := claim.Created
	if created.IsZero() {
		created = time.Now()
		claim.Created = created
	}
	month := created.Format("2006-01")
	return filepath.Join(s.baseDir, month, claim.ID+".md")
}

func (s *Store) Create(claim *claims.Claim) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.claims[claim.ID]; exists {
		return errors.New("claim already exists")
	}

	path := s.getClaimPath(claim)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}

	data, err := claims.Write(claim)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return err
	}

	s.claims[claim.ID] = path
	s.indexRelationsUnsafe(claim)

	return nil
}

func (s *Store) Read(id string) (*claims.Claim, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	path, exists := s.claims[id]
	if !exists {
		return nil, ErrNotFound
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	return claims.Parse(data)
}

func (s *Store) Update(claim *claims.Claim) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	oldPath, exists := s.claims[claim.ID]
	if !exists {
		return ErrNotFound
	}

	// Read old claim to remove its relations
	oldData, err := os.ReadFile(oldPath)
	if err == nil {
		if oldClaim, err := claims.Parse(oldData); err == nil {
			s.removeRelationsUnsafe(oldClaim)
		}
	}

	newPath := s.getClaimPath(claim)
	if newPath != oldPath {
		if err := os.MkdirAll(filepath.Dir(newPath), 0755); err != nil {
			return err
		}
	}

	data, err := claims.Write(claim)
	if err != nil {
		return err
	}

	if err := os.WriteFile(newPath, data, 0644); err != nil {
		return err
	}

	if newPath != oldPath {
		os.Remove(oldPath)
	}

	s.claims[claim.ID] = newPath
	s.indexRelationsUnsafe(claim)

	return nil
}

func (s *Store) Delete(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	path, exists := s.claims[id]
	if !exists {
		return ErrNotFound
	}

	// Read to remove relations
	data, err := os.ReadFile(path)
	if err == nil {
		if claim, err := claims.Parse(data); err == nil {
			s.removeRelationsUnsafe(claim)
		}
	}

	if err := os.Remove(path); err != nil {
		return err
	}

	delete(s.claims, id)

	return nil
}

// GetInverseRelations returns the inverse relations for a given claim ID.
func (s *Store) GetInverseRelations(id string) InverseRelations {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var inv InverseRelations
	rel, exists := s.relationIndex[id]
	if !exists {
		return inv
	}

	inv.SupportedBy = append([]string(nil), rel["supports"]...)
	inv.ContradictedBy = append([]string(nil), rel["contradicts"]...)
	inv.SupersededBy = append([]string(nil), rel["supersedes"]...)
	inv.ExplainedBy = append([]string(nil), rel["explains"]...)
	inv.RefinedBy = append([]string(nil), rel["refines"]...)
	inv.DependedOnBy = append([]string(nil), rel["depends_on"]...)

	return inv
}

// ListAll returns all claims in the store.
func (s *Store) ListAll() ([]*claims.Claim, error) {
	s.mu.RLock()
	var ids []string
	for id := range s.claims {
		ids = append(ids, id)
	}
	s.mu.RUnlock()

	var out []*claims.Claim
	for _, id := range ids {
		c, err := s.Read(id)
		if err == nil {
			out = append(out, c)
		}
	}
	return out, nil
}
