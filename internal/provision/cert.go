package provision

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"devhelper/internal/app"
	"devhelper/internal/platform"
)

func CertInit(cfg app.Config) error {
	_, err := runHost(cfg.MkcertCli, "-install")
	if err != nil {
		return fmt.Errorf("mkcert -install failed: %w", err)
	}

	certsDirRuntime := filepath.ToSlash(filepath.Join(cfg.NginxExternalRoot, "certs"))
	hostCertsDir := certsDirRuntime
	if runtime.GOOS == "windows" {
		hostCertsDir = platform.WSLToHost(cfg, certsDirRuntime)
	}
	_ = os.MkdirAll(hostCertsDir, 0o755)
	return nil
}

func CertIssue(cfg app.Config, domain string) error {
	domain = strings.TrimSpace(domain)
	if domain == "" {
		return fmt.Errorf("domain required")
	}

	certDirRuntime := filepath.ToSlash(filepath.Join(cfg.NginxExternalRoot, "certs", domain))
	hostCertDir := certDirRuntime
	if runtime.GOOS == "windows" {
		hostCertDir = platform.WSLToHost(cfg, certDirRuntime)
	}
	if err := os.MkdirAll(hostCertDir, 0o755); err != nil {
		return err
	}

	certFile := filepath.Join(hostCertDir, "cert.pem")
	keyFile := filepath.Join(hostCertDir, "key.pem")

	_, err := runHost(cfg.MkcertCli, "-cert-file", certFile, "-key-file", keyFile, domain)
	if err != nil {
		return fmt.Errorf("mkcert issue failed: %w", err)
	}
	return nil
}

func runHost(exe string, args ...string) (string, error) {
	if strings.TrimSpace(exe) == "" {
		exe = "mkcert"
	}
	cmd := exec.Command(exe, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, string(out))
	}
	return string(out), nil
}
