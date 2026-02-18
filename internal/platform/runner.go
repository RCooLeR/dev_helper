package platform

import (
	"bytes"
	"fmt"
	"os/exec"
	"runtime"
	"strings"

	"devhelper/internal/app"
)

type Runner struct{ cfg app.Config }

func NewRunner(cfg app.Config) *Runner { return &Runner{cfg: cfg} }

// Shell executes a command via the host shell.
//
// NOTE: We avoid using this for DB operations (we run mysql/psql directly with argv),
// because quoting gets painful on Windows.
func (r *Runner) Shell(command string) (string, error) {
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.Command("cmd.exe", "/c", command)
	} else {
		cmd = exec.Command("bash", "-lc", command)
	}
	var out, errb bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errb
	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("command failed: %w; stderr=%s", err, strings.TrimSpace(errb.String()))
	}
	return strings.TrimSpace(out.String()), nil
}

func (r *Runner) Shellf(format string, args ...any) (string, error) {
	return r.Shell(fmt.Sprintf(format, args...))
}
