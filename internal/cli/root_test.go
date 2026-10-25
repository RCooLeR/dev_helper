package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"devhelper/internal/app"
	"devhelper/internal/store"

	urfave "github.com/urfave/cli/v3"
)

func testCommand(t *testing.T) (*urfave.Command, *bytes.Buffer) {
	t.Helper()
	root := t.TempDir()
	a := &app.App{RepoRoot: root, Cfg: app.Config{StoreFile: filepath.Join(root, "store.json")}}
	cmd := newCommand(a)
	output := new(bytes.Buffer)
	cmd.Writer = output
	cmd.ErrWriter = io.Discard
	return cmd, output
}

func TestCommandHelp(t *testing.T) {
	for _, path := range []string{"", "init", "serve", "project", "project create", "project drop", "project list", "db ensure", "db import", "cert init", "cert issue"} {
		t.Run(path, func(t *testing.T) {
			cmd, output := testCommand(t)
			args := append([]string{"devhelper"}, strings.Fields(path)...)
			args = append(args, "--help")
			if err := cmd.Run(t.Context(), args); err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(output.String(), "USAGE:") {
				t.Fatalf("missing command help: %s", output)
			}
		})
	}
}

func TestRequiredFlagsPreventActions(t *testing.T) {
	for _, path := range []string{"project create", "project drop", "db ensure", "db import", "cert issue"} {
		t.Run(path, func(t *testing.T) {
			cmd, _ := testCommand(t)
			leaf := cmd
			for _, name := range strings.Fields(path) {
				leaf = leaf.Command(name)
			}
			called := false
			leaf.Action = func(context.Context, *urfave.Command) error {
				called = true
				return nil
			}
			err := cmd.Run(t.Context(), append([]string{"devhelper"}, strings.Fields(path)...))
			if err == nil || called {
				t.Fatalf("required flags did not stop action: error=%v called=%v", err, called)
			}
		})
	}
}

func TestProjectCreatePreservesFlagsAndContext(t *testing.T) {
	cmd, _ := testCommand(t)
	type contextKey struct{}
	ctx := context.WithValue(t.Context(), contextKey{}, "caller")
	called := false
	cmd.Command("project").Command("create").Action = func(ctx context.Context, cmd *urfave.Command) error {
		called = true
		if ctx.Value(contextKey{}) != "caller" {
			t.Error("caller context was not propagated")
		}
		for flag, want := range map[string]string{
			"company": "acme", "project": "shop", "type": "wordpress", "domain": "shop.acme.local",
			"php": "8.4", "db": "mysql8.4", "import-file": "dump with spaces.sql",
		} {
			if got := cmd.String(flag); got != want {
				t.Errorf("--%s = %q, want %q", flag, got, want)
			}
		}
		if !cmd.Bool("import") {
			t.Error("--import was not parsed")
		}
		return nil
	}
	err := cmd.Run(ctx, []string{"devhelper", "project", "create", "--company", "acme", "--project", "shop", "--type", "wordpress", "--domain", "shop.acme.local", "--php", "8.4", "--db", "mysql8.4", "--import-file", "dump with spaces.sql", "--import"})
	if err != nil || !called {
		t.Fatalf("create command did not run: error=%v called=%v", err, called)
	}
}

func TestCanceledContextPreventsCommandAction(t *testing.T) {
	cmd, _ := testCommand(t)
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	called := false
	cmd.Command("init").Action = func(context.Context, *urfave.Command) error {
		called = true
		return nil
	}
	if err := cmd.Run(ctx, []string{"devhelper", "init"}); !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context cancellation", err)
	}
	if called {
		t.Fatal("canceled command ran its action")
	}
}

func TestProjectListWritesJSONToCommandWriter(t *testing.T) {
	cmd, output := testCommand(t)
	if err := cmd.Run(t.Context(), []string{"devhelper", "project", "list"}); err != nil {
		t.Fatal(err)
	}
	var projects []store.Project
	if err := json.Unmarshal(output.Bytes(), &projects); err != nil {
		t.Fatalf("invalid list output %q: %v", output, err)
	}
	if projects == nil || len(projects) != 0 {
		t.Fatalf("expected an empty JSON array, got %s", output)
	}
}

func TestProjectListReportsInvalidStore(t *testing.T) {
	root := t.TempDir()
	storePath := filepath.Join(root, "store.json")
	if err := os.WriteFile(storePath, []byte("broken json"), 0o600); err != nil {
		t.Fatal(err)
	}
	cmd := newCommand(&app.App{Cfg: app.Config{StoreFile: storePath}})
	cmd.Writer = io.Discard
	cmd.ErrWriter = io.Discard
	if err := cmd.Run(t.Context(), []string{"devhelper", "project", "list"}); err == nil {
		t.Fatal("invalid project store was reported as success")
	}
}

func TestUnknownFlagIsRejected(t *testing.T) {
	cmd, _ := testCommand(t)
	err := cmd.Run(t.Context(), []string{"devhelper", "project", "create", "--copmany", "acme"})
	if err == nil {
		t.Fatal("unknown flag was accepted")
	}
}

func TestServeDefaultsToLoopbackAndAllowsExplicitAddress(t *testing.T) {
	for _, explicit := range []string{"", ":8787"} {
		t.Run(explicit, func(t *testing.T) {
			cmd, _ := testCommand(t)
			got := ""
			cmd.Command("serve").Action = func(_ context.Context, cmd *urfave.Command) error {
				got = cmd.String("http")
				return nil
			}
			args := []string{"devhelper", "serve"}
			want := "127.0.0.1:8787"
			if explicit != "" {
				args = append(args, "--http", explicit)
				want = explicit
			}
			if err := cmd.Run(t.Context(), args); err != nil {
				t.Fatal(err)
			}
			if got != want {
				t.Fatalf("listen address = %q, want %q", got, want)
			}
		})
	}
}

func TestDBEnsureRejectsMissingOrDatabaseFreeProject(t *testing.T) {
	for _, saved := range []bool{false, true} {
		name := "missing"
		wantError := "project acme/shop not found"
		if saved {
			name = "no database"
			wantError = "project acme/shop has no database"
		}
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			storePath := filepath.Join(root, "store.json")
			st, err := store.Open(storePath)
			if err != nil {
				t.Fatal(err)
			}
			if saved {
				if _, err := st.Upsert(store.Project{Company: "acme", Name: "shop", DB: "none"}); err != nil {
					t.Fatal(err)
				}
			}
			// If validation regresses, configured tools remain nonexistent so
			// the test cannot invoke a developer's installed Docker/DB clients.
			disabledTool := filepath.Join(root, "tools-disabled")
			cmd := newCommand(&app.App{Cfg: app.Config{
				StoreFile: storePath, DockerCli: disabledTool, MySQLCli: disabledTool, PSQLCli: disabledTool,
			}})
			output := new(bytes.Buffer)
			cmd.Writer, cmd.ErrWriter = output, io.Discard
			err = cmd.Run(t.Context(), []string{"devhelper", "db", "ensure", "--company", "acme", "--project", "shop"})
			if err == nil || !strings.Contains(err.Error(), wantError) {
				t.Fatalf("error = %v, want %q", err, wantError)
			}
			if strings.Contains(output.String(), `"ok": true`) {
				t.Fatalf("failed repair emitted a success response: %s", output)
			}
		})
	}
}
