package platform

import (
	"bufio"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"devhelper/internal/app"

	"github.com/rs/zerolog/log"
)

// GetComposeEnv returns a value from (1) process env, then (2) <composeDir>/.env,
// falling back to the provided default.
func GetComposeEnv(cfg app.Config, key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	m, err := ReadComposeEnv(cfg)
	if err != nil {
		log.Err(err).Msg("failed to read env")
		return fallback
	}
	if v, ok := m[key]; ok && strings.TrimSpace(v) != "" {
		return v
	}
	return fallback
}

// ReadComposeEnv reads <composeDir>/.env into a map.
func ReadComposeEnv(cfg app.Config) (map[string]string, error) {
	envPath := filepath.ToSlash(filepath.Join(cfg.ComposeDir, ".env"))
	var content string
	if runtime.GOOS == "windows" {
		r := NewRunner(cfg)
		out, err := r.Shellf("test -f %q && cat %q || true", envPath, envPath)
		if err != nil {
			// If WSL path is not ready yet, just surface empty map.
			log.Err(err).Msg("failed to read env")
			return map[string]string{}, nil
		}
		content = out
	} else {
		b, err := os.ReadFile(envPath)
		if err != nil {
			log.Err(err).Msg("failed to read env")
			return map[string]string{}, err
		}
		content = string(b)
	}

	m := make(map[string]string)
	s := bufio.NewScanner(strings.NewReader(content))
	for s.Scan() {
		line := strings.TrimSpace(s.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.TrimSpace(line[i+1:])
		v = strings.Trim(v, "\"'")
		m[k] = v
	}
	return m, nil
}
