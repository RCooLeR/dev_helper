package platform

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

func hostsPath() string {
	if runtime.GOOS == "windows" {
		root := os.Getenv("SystemRoot")
		if root == "" {
			root = `C:\Windows`
		}
		return filepath.Join(root, "System32", "drivers", "etc", "hosts")
	}
	return "/etc/hosts"
}

func hostsLineHasDomain(line, domain string) bool {
	fields := strings.Fields(line)
	if len(fields) < 2 {
		return false
	}
	for _, field := range fields[1:] {
		if field == "#" {
			break
		}
		if strings.HasPrefix(field, "#") {
			break
		}
		if field == domain {
			return true
		}
	}
	return false
}

func AddHost(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil
	}

	p := hostsPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read hosts (%s): %w", p, err)
	}
	s := string(b)

	for _, line := range strings.Split(s, "\n") {
		l := strings.TrimSpace(line)
		if l == "" || strings.HasPrefix(l, "#") {
			continue
		}
		if hostsLineHasDomain(l, domain) {
			return nil
		}
	}

	var buf bytes.Buffer
	buf.WriteString(s)
	if !strings.HasSuffix(s, "\n") {
		buf.WriteString("\n")
	}
	buf.WriteString(fmt.Sprintf("127.0.0.1 %s # devhelper\n", domain))
	buf.WriteString(fmt.Sprintf("::1 %s # devhelper\n", domain))

	if err := os.WriteFile(p, buf.Bytes(), 0o644); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("write hosts (%s): permission denied (run as admin/sudo)", p)
		}
		return fmt.Errorf("write hosts (%s): %w", p, err)
	}
	return nil
}

// RemoveHost removes previously added devhelper host entries for a domain.
// It only removes lines that contain both the domain and the "devhelper" marker.
func RemoveHost(domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return nil
	}

	p := hostsPath()
	b, err := os.ReadFile(p)
	if err != nil {
		return fmt.Errorf("read hosts (%s): %w", p, err)
	}

	lines := strings.Split(string(b), "\n")
	out := make([]string, 0, len(lines))
	for _, line := range lines {
		if hostsLineHasDomain(line, domain) && strings.Contains(line, "devhelper") {
			continue
		}
		out = append(out, line)
	}

	nb := []byte(strings.Join(out, "\n"))
	if err := os.WriteFile(p, nb, 0o644); err != nil {
		if errors.Is(err, os.ErrPermission) {
			return fmt.Errorf("write hosts (%s): permission denied (run as admin/sudo)", p)
		}
		return fmt.Errorf("write hosts (%s): %w", p, err)
	}
	return nil
}
