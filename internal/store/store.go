package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Project struct {
	ID      string `json:"id"`
	Company string `json:"company"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Domain  string `json:"domain"`
	PHP     string `json:"php"`
	DB      string `json:"db"`

	DBName string `json:"db_name,omitempty"`
	DBUser string `json:"db_user,omitempty"`
	DBPass string `json:"db_pass,omitempty"`

	RootPath  string `json:"root_path"`
	HostPath  string `json:"host_path"`
	NginxConf string `json:"nginx_conf"`

	CreatedAt time.Time `json:"created_at"`
}

type State struct {
	Projects []Project `json:"projects"`
}

type Store struct {
	mu   sync.Mutex
	path string
	st   State
}

func Open(path string) (*Store, error) {
	s := &Store{path: path}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			s.st = State{}
			return nil
		}
		return err
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return err
	}
	s.st = st
	return nil
}

func (s *Store) saveLocked() error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(s.st, "", "  ")
	return os.WriteFile(s.path, b, 0o644)
}

func newID() string {
	var b [12]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func (s *Store) List() []Project {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Project, len(s.st.Projects))
	copy(out, s.st.Projects)
	return out
}

func (s *Store) Get(company, project string) (Project, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.st.Projects {
		if p.Company == company && p.Name == project {
			return p, true
		}
	}
	return Project{}, false
}

func (s *Store) Upsert(p Project) (Project, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if p.ID == "" {
		p.ID = newID()
	}
	if p.CreatedAt.IsZero() {
		p.CreatedAt = time.Now()
	}

	for i, ex := range s.st.Projects {
		if ex.Company == p.Company && ex.Name == p.Name {
			p.ID = ex.ID
			p.CreatedAt = ex.CreatedAt
			s.st.Projects[i] = p
			return p, s.saveLocked()
		}
	}
	s.st.Projects = append(s.st.Projects, p)
	return p, s.saveLocked()
}

// Delete removes a project from the store.
// It returns the deleted project and whether it existed.
func (s *Store) Delete(company, project string) (Project, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, p := range s.st.Projects {
		if p.Company == company && p.Name == project {
			// remove index i
			s.st.Projects = append(s.st.Projects[:i], s.st.Projects[i+1:]...)
			return p, true, s.saveLocked()
		}
	}
	return Project{}, false, nil
}
