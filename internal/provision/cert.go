package provision

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	"devhelper/internal/app"
	"devhelper/internal/platform"

	"github.com/rs/zerolog/log"
)

func CertInit(cfg app.Config) error {
	return CertInitContext(context.Background(), cfg)
}

func CertInitContext(ctx context.Context, cfg app.Config) error {
	log.Info().Msg("CertInit called")
	_, err := runHostContext(ctx, cfg.MkcertCli, "-install")
	if err != nil {
		log.Error().Err(err).Msg("mkcert -install failed")
		return fmt.Errorf("mkcert -install failed: %w", err)
	}
	log.Info().Msg("CertInit succeeded")
	certsDirRuntime := filepath.ToSlash(filepath.Join(cfg.NginxExternalRoot, "certs"))
	hostCertsDir := certsDirRuntime
	if runtime.GOOS == "windows" {
		hostCertsDir = platform.WSLToHost(cfg, certsDirRuntime)
	}
	return os.MkdirAll(hostCertsDir, 0o755)
}

func CertIssue(cfg app.Config, domain string) error {
	return CertIssueContext(context.Background(), cfg, domain)
}

func CertIssueContext(ctx context.Context, cfg app.Config, domain string) error {
	domain = strings.TrimSpace(domain)
	if err := ctx.Err(); err != nil {
		return err
	}
	if err := validateDomain(domain); err != nil {
		return err
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
	log.Info().Msgf("Creating cert.pem and key.pem for domain %s", domain)
	_, err := runHostContext(ctx, cfg.MkcertCli, "-cert-file", certFile, "-key-file", keyFile, domain)
	if err != nil {
		log.Error().Err(err).Msg("mkcert -cert failed")
		return fmt.Errorf("mkcert issue failed: %w", err)
	}
	log.Info().Msgf("Created cert.pem and key.pem for domain %s", domain)
	//set 644 permissions for cert and key files
	if runtime.GOOS != "windows" {
		if err := os.Chmod(certFile, 0o644); err != nil {
			return fmt.Errorf("failed to set permissions for cert.pem: %w", err)
		}
		if err := os.Chmod(keyFile, 0o644); err != nil {
			return fmt.Errorf("failed to set permissions for key.pem: %w", err)
		}
	}
	return nil
}

func runHost(exe string, args ...string) (string, error) {
	return runHostContext(context.Background(), exe, args...)
}

func runHostContext(ctx context.Context, exe string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Minute)
	defer cancel()
	if strings.TrimSpace(exe) == "" {
		exe = "mkcert"
	}
	cmd := exec.CommandContext(ctx, exe, args...)
	cmd.WaitDelay = 2 * time.Second
	out, err := cmd.CombinedOutput()
	if ctx.Err() != nil {
		return string(out), ctx.Err()
	}
	if err != nil {
		return string(out), fmt.Errorf("%w: %s", err, string(out))
	}
	return string(out), nil
}
