package platform

import (
	"fmt"
	"os"
	"path/filepath"

	"devhelper/internal/app"

	"github.com/compose-spec/compose-go/v2/dotenv"
	"github.com/rs/zerolog/log"
)

// GetComposeEnv returns a value from (1) process env, then (2) <composeDir>/.env,
// falling back to the provided default.
func GetComposeEnv(cfg app.Config, key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok {
		return v
	}
	m, err := ReadComposeEnv(cfg)
	if err != nil {
		log.Err(err).Msg("failed to read env")
		return fallback
	}
	if v, ok := m[key]; ok {
		return v
	}
	return fallback
}

// ReadComposeEnv reads <composeDir>/.env into a map.
func ReadComposeEnv(cfg app.Config) (map[string]string, error) {
	// ComposeDir is always the host directory that contains docker-compose.yml.
	// On Windows that means a normal path such as D:\Development\projects, not
	// a WSL path. Reading the file directly avoids shell/quoting differences
	// and fixes older behavior that tried to run `test -f && cat` via cmd.exe.
	envPath := filepath.Join(cfg.ComposeDir, ".env")
	f, err := os.Open(envPath)
	if err != nil {
		if os.IsNotExist(err) {
			return map[string]string{}, nil
		}
		return nil, err
	}
	defer f.Close()
	m, err := dotenv.ParseWithLookup(f, os.LookupEnv)
	if err != nil {
		return nil, fmt.Errorf("parse compose .env: %w", err)
	}
	return m, nil
}
