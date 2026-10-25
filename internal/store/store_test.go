package store

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

func openTestStore(t *testing.T, path string) *Store {
	t.Helper()
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestIndependentStoresObserveAndPreserveUpdates(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "projects.json")
	a := openTestStore(t, path)
	b := openTestStore(t, path)
	projects, err := a.List()
	if err != nil || projects == nil || len(projects) != 0 {
		t.Fatalf("empty List() = %#v, %v", projects, err)
	}
	first, err := a.Upsert(Project{Company: "company", Name: "first", DBPass: "secret"})
	if err != nil {
		t.Fatal(err)
	}
	if first.ID == "" || first.CreatedAt.IsZero() {
		t.Fatalf("new project missing identity: %+v", first)
	}
	if _, err := b.Upsert(Project{Company: "company", Name: "second"}); err != nil {
		t.Fatal(err)
	}
	projects, err = a.List()
	if err != nil || len(projects) != 2 {
		t.Fatalf("List() lost another handle's project: %#v, %v", projects, err)
	}
	projects[0].Name = "caller-mutation"
	updated, err := b.Upsert(Project{Company: "company", Name: "first", DBPass: "new-secret"})
	if err != nil {
		t.Fatal(err)
	}
	if updated.ID != first.ID || !updated.CreatedAt.Equal(first.CreatedAt) {
		t.Fatalf("update changed project identity: %+v -> %+v", first, updated)
	}
	got, ok, err := a.Get("company", "first")
	if err != nil || !ok || got.DBPass != "new-secret" {
		t.Fatalf("Get() returned stale state: %+v, %v, %v", got, ok, err)
	}
	if _, ok, err := a.Delete("company", "second"); err != nil || !ok {
		t.Fatalf("Delete() = %v, %v", ok, err)
	}
	if _, ok, err := b.Get("company", "second"); err != nil || ok {
		t.Fatalf("deleted project still visible: %v, %v", ok, err)
	}
	if _, ok, err := b.Delete("company", "missing"); err != nil || ok {
		t.Fatalf("missing Delete() = %v, %v", ok, err)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("credential store permissions = %o, want 600", info.Mode().Perm())
		}
	}
}

func TestFailedSavePreservesPersistedProjects(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	s := openTestStore(t, path)
	original, err := s.Upsert(Project{Company: "company", Name: "app", DBPass: "original"})
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"app", "new-app"} {
		t.Run(name, func(t *testing.T) {
			// JSON v2 rejects invalid UTF-8 instead of silently changing credentials.
			if _, err := s.Upsert(Project{Company: "company", Name: name, DBPass: "\xff"}); err == nil {
				t.Fatal("invalid credential unexpectedly saved")
			}
			after, err := os.ReadFile(path)
			if err != nil || !bytes.Equal(before, after) {
				t.Fatalf("failed save changed disk contents: %q, %v", after, err)
			}
			got, ok, err := s.Get("company", "app")
			if err != nil || !ok || got.ID != original.ID || got.DBPass != original.DBPass || !got.CreatedAt.Equal(original.CreatedAt) {
				t.Fatalf("failed save changed visible state: %+v, %v, %v", got, ok, err)
			}
			projects, err := s.List()
			if err != nil || len(projects) != 1 {
				t.Fatalf("failed insertion retained in store: %+v, %v", projects, err)
			}
		})
	}
	matches, err := filepath.Glob(filepath.Join(filepath.Dir(path), ".devhelper-store-*"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary store leaked: %v, %v", matches, err)
	}
}

func TestCorruptStoreIsNotOverwritten(t *testing.T) {
	for _, content := range []string{
		`{"projects":`,
		`{"projects":[],"projects":[]}`,
		"{\"projects\":[{\"name\":\"\xff\"}]}",
	} {
		t.Run(fmt.Sprintf("%q", content), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "projects.json")
			s := openTestStore(t, path)
			if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
				t.Fatal(err)
			}
			if _, err := Open(path); err == nil {
				t.Fatal("Open accepted corrupt store")
			}
			if _, err := s.List(); err == nil {
				t.Fatal("List hid store corruption")
			}
			if _, _, err := s.Get("company", "app"); err == nil {
				t.Fatal("Get hid store corruption")
			}
			if _, err := s.Upsert(Project{Company: "company", Name: "app"}); err == nil {
				t.Fatal("Upsert replaced corrupt store")
			}
			if _, _, err := s.Delete("company", "app"); err == nil {
				t.Fatal("Delete hid store corruption")
			}
			after, err := os.ReadFile(path)
			if err != nil || string(after) != content {
				t.Fatalf("corrupted file was changed: %q, %v", after, err)
			}
		})
	}
}

func TestConcurrentStoreHandles(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	stores := make([]*Store, 8)
	for i := range stores {
		stores[i] = openTestStore(t, path)
	}
	var group sync.WaitGroup
	for i, s := range stores {
		group.Go(func() {
			for j := range 10 {
				if _, err := s.Upsert(Project{Company: "company", Name: fmt.Sprintf("%d-%d", i, j)}); err != nil {
					t.Error(err)
					return
				}
			}
		})
	}
	group.Wait()
	projects, err := stores[0].List()
	if err != nil || len(projects) != 80 {
		t.Fatalf("concurrent writes lost projects: count=%d, err=%v", len(projects), err)
	}
}

// This helper runs in separate test processes so the regression exercises real
// OS locking, not only a mutex shared by handles in one process.
func TestStoreProcessWriter(t *testing.T) {
	path := os.Getenv("DEVHELPER_TEST_STORE_PATH")
	if path == "" {
		t.Skip("subprocess helper")
	}
	s := openTestStore(t, path)
	worker := os.Getenv("DEVHELPER_TEST_STORE_WORKER")
	for j := range 15 {
		if _, err := s.Upsert(Project{Company: worker, Name: strconv.Itoa(j)}); err != nil {
			t.Fatal(err)
		}
	}
}

func TestConcurrentStoreProcesses(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "projects.json")
	const count = 4
	var commands [count]*exec.Cmd
	var outputs [count]bytes.Buffer
	for i := range commands {
		cmd := exec.CommandContext(t.Context(), executable, "-test.run=^TestStoreProcessWriter$")
		cmd.Env = append(os.Environ(), "DEVHELPER_TEST_STORE_PATH="+path, "DEVHELPER_TEST_STORE_WORKER="+strconv.Itoa(i))
		cmd.Stdout = &outputs[i]
		cmd.Stderr = &outputs[i]
		if err := cmd.Start(); err != nil {
			t.Fatal(err)
		}
		commands[i] = cmd
	}
	for i, cmd := range commands {
		if err := cmd.Wait(); err != nil {
			t.Errorf("writer %d: %v\n%s", i, err, outputs[i].String())
		}
	}
	s := openTestStore(t, path)
	projects, err := s.List()
	if err != nil || len(projects) != count*15 {
		t.Fatalf("concurrent processes lost projects: count=%d, err=%v", len(projects), err)
	}
}

func TestOperationLockCancellationAndIndependentStoreAccess(t *testing.T) {
	path := filepath.Join(t.TempDir(), "projects.json")
	a := openTestStore(t, path)
	b := openTestStore(t, path)
	unlock, err := a.LockOperations(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	defer unlock()
	if _, err := b.Upsert(Project{Company: "company", Name: "app"}); err != nil {
		t.Fatal(err)
	}
	if _, ok, err := a.Get("company", "app"); err != nil || !ok {
		t.Fatalf("operation lock blocked ordinary store access: %v, %v", ok, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 50*time.Millisecond)
	defer cancel()
	if otherUnlock, err := b.LockOperations(ctx); !errors.Is(err, context.DeadlineExceeded) {
		if otherUnlock != nil {
			otherUnlock()
		}
		t.Fatalf("waiting for operation lock: got %v, want deadline exceeded", err)
	}
	unlock()
	otherUnlock, err := b.LockOperations(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	otherUnlock()
	otherUnlock() // Cleanup may release twice without releasing another holder.
}
