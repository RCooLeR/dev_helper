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
		if strings.Contains(" "+l+" ", " "+domain+" ") || strings.HasSuffix(l, " "+domain) {
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
