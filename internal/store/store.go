package store

import (
	"context"
	"crypto/rand"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"
)

// DatabaseConnection records an application's additional database dependencies.
// Passwords stay in the application's own configuration, not in this inventory.
type DatabaseConnection struct {
	Name     string `json:"name"`
	Engine   string `json:"engine"`
	Host     string `json:"host,omitempty"`
	Port     int    `json:"port,omitempty"`
	Database string `json:"database,omitempty"`
	User     string `json:"user,omitempty"`
	External bool   `json:"external,omitempty"`
	Source   string `json:"source,omitempty"`
}

type Project struct {
	ID      string `json:"id"`
	Company string `json:"company"`
	Name    string `json:"name"`
	Type    string `json:"type"`
	Domain  string `json:"domain"`
	PHP     string `json:"php"`
	DB      string `json:"db"`

	DBName        string               `json:"db_name,omitempty"`
	DBUser        string               `json:"db_user,omitempty"`
	DBPass        string               `json:"db_pass,omitempty"`
	DBHost        string               `json:"db_host,omitempty"`
	DBPort        int                  `json:"db_port,omitempty"`
	DBExternal    bool                 `json:"db_external,omitempty"`
	DBConnections []DatabaseConnection `json:"db_connections,omitempty"`

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
}

// Open loads the JSON store from disk. Missing files are treated as an empty
// store so first-run setup does not require precreating data/devhelper.store.json.
func Open(path string) (*Store, error) {
	abs, err := filepath.Abs(path)
	if err != nil {
		return nil, err
	}
	s := &Store{path: abs}
	if _, err := s.List(); err != nil {
		return nil, err
	}
	return s, nil
}

// withLock serializes the whole read/modify/write transaction, including other
// CLI/server processes. Lock the stable sidecar, not the replaced JSON inode.
// The sidecar must remain on disk: unlinking it could let two writers lock
// different files bearing the same name.
func (s *Store) withLock(fn func() error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	unlock, err := acquireLock(context.Background(), s.path+".lock")
	if err != nil {
		return err
	}
	defer unlock()
	return fn()
}

// LockOperations coordinates multi-step create/drop/import operations across
// CLI and HTTP processes. The separate lock lets callers read and commit store
// records while holding it. A canceled context interrupts lock acquisition.
func (s *Store) LockOperations(ctx context.Context) (func(), error) {
	return acquireLock(ctx, s.path+".operations.lock")
}

func acquireLock(ctx context.Context, path string) (func(), error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create store directory: %w", err)
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open store lock: %w", err)
	}
	if err := lockFile(ctx, f); err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("lock store: %w", err)
	}
	return sync.OnceFunc(func() {
		unlockFile(f)
		_ = f.Close()
	}), nil
}

func lockFile(ctx context.Context, f *os.File) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		locked, err := tryLockFile(f)
		if locked || err != nil {
			return err
		}
		timer := time.NewTimer(25 * time.Millisecond)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
}

func (s *Store) loadLocked() (State, error) {
	b, err := os.ReadFile(s.path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return State{Projects: []Project{}}, nil
		}
		return State{}, fmt.Errorf("read store: %w", err)
	}
	var st State
	if err := json.Unmarshal(b, &st); err != nil {
		return State{}, fmt.Errorf("decode store: %w", err)
	}
	if st.Projects == nil {
		st.Projects = []Project{}
	}
	return st, nil
}

func (s *Store) saveLocked(st State) error {
	b, err := json.Marshal(&st, jsontext.WithIndent("  "))
	if err != nil {
		return fmt.Errorf("encode store: %w", err)
	}
	// Store credentials in a private temporary file in the same directory, then
	// replace only after a complete write and fsync. A failed write never truncates
	// the previous store or leaves an uncommitted mutation visible in memory.
	f, err := os.CreateTemp(filepath.Dir(s.path), ".devhelper-store-*")
	if err != nil {
		return fmt.Errorf("create temporary store: %w", err)
	}
	tmp := f.Name()
	defer os.Remove(tmp)
	defer f.Close()
	if _, err := f.Write(b); err != nil {
		return fmt.Errorf("write store: %w", err)
	}
	if err := f.Sync(); err != nil {
		return fmt.Errorf("sync store: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("close store: %w", err)
	}
	if err := replaceFile(tmp, s.path); err != nil {
		return fmt.Errorf("replace store: %w", err)
	}
	return nil
}

// newID returns a compact random identifier for UI keys and future references.
// It is not meant to encode business meaning; company+project is still the
// human-facing unique key.
func newID() string {
	return rand.Text()
}

// List reads current disk state, including changes made by another process.
func (s *Store) List() ([]Project, error) {
	var st State
	err := s.withLock(func() error {
		var err error
		st, err = s.loadLocked()
		return err
	})
	return st.Projects, err
}

func (s *Store) Get(company, project string) (Project, bool, error) {
	projects, err := s.List()
	if err != nil {
		return Project{}, false, err
	}
	for _, p := range projects {
		if p.Company == company && p.Name == project {
			return p, true, nil
		}
	}
	return Project{}, false, nil
}

func (s *Store) Upsert(p Project) (Project, error) {
	err := s.withLock(func() error {
		st, err := s.loadLocked()
		if err != nil {
			return err
		}
		index := slices.IndexFunc(st.Projects, func(ex Project) bool {
			return ex.Company == p.Company && ex.Name == p.Name
		})
		if index >= 0 {
			p.ID = st.Projects[index].ID
			p.CreatedAt = st.Projects[index].CreatedAt
		}
		if p.ID == "" {
			p.ID = newID()
		}
		if p.CreatedAt.IsZero() {
			p.CreatedAt = time.Now()
		}
		if index >= 0 {
			st.Projects[index] = p
		} else {
			st.Projects = append(st.Projects, p)
		}
		return s.saveLocked(st)
	})
	if err != nil {
		return Project{}, err
	}
	return p, nil
}

// Delete removes a project from the store.
// It returns the deleted project and whether it existed.
func (s *Store) Delete(company, project string) (Project, bool, error) {
	var deleted Project
	found := false
	err := s.withLock(func() error {
		st, err := s.loadLocked()
		if err != nil {
			return err
		}
		for i, p := range st.Projects {
			if p.Company == company && p.Name == project {
				deleted, found = p, true
				st.Projects = slices.Delete(st.Projects, i, i+1)
				return s.saveLocked(st)
			}
		}
		return nil
	})
	if err != nil {
		return Project{}, false, err
	}
	return deleted, found, nil
}
