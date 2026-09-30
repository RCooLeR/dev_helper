package platform

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"devhelper/internal/app"

	"github.com/compose-spec/compose-go/v2/dotenv"
)

const composeEnvBegin = "# BEGIN devhelper managed environment"
const composeEnvEnd = "# END devhelper managed environment"

// WriteEnv writes a .env file next to docker-compose.yml so `docker compose` can resolve paths and passwords.
func WriteEnv(cfg app.Config) error {
	if cfg.ComposeDir == "" {
		return fmt.Errorf("compose_dir is empty")
	}
	envPath := filepath.Join(cfg.ComposeDir, ".env")

	values := []struct{ key, value string }{
		{"APPS_ROOT", cfg.AppsRoot},
		{"DATA_ROOT", cfg.DataRoot},
		{"NGINX_CONF_ROOT", cfg.NginxConfRoot},
		{"NGINX_EXTERNAL_ROOT", cfg.NginxExternalRoot},
		{"MYSQL_ROOT_PASSWORD", cfg.MySQLRootPass},
		{"POSTGRES_SUPER_PASSWORD", cfg.PostgresSuperPass},
	}
	var content strings.Builder
	content.WriteString(composeEnvBegin + "\n")
	for _, entry := range values {
		if !utf8.ValidString(entry.value) || strings.ContainsRune(entry.value, 0) {
			return fmt.Errorf("%s contains invalid environment value characters", entry.key)
		}
		fmt.Fprintf(&content, "%s=%s\n", entry.key, quoteComposeValue(entry.value))
	}
	content.WriteString(composeEnvEnd + "\n")
	existing, err := os.ReadFile(envPath)
	if err != nil && !os.IsNotExist(err) {
		return err
	}
	merged, err := mergeComposeEnv(string(existing), content.String())
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		return err
	}
	return os.WriteFile(envPath, []byte(merged), 0o600)
}

// Preserve user-owned entries verbatim, including comments, interpolation and
// multiline values. Legacy files receive one managed block at the end; future
// runs replace only that block. Validate before touching the existing file.
func mergeComposeEnv(existing, generated string) (string, error) {
	parse := func(content string) error {
		_, err := dotenv.ParseWithLookup(strings.NewReader(content), os.LookupEnv)
		return err
	}
	if err := parse(existing); err != nil {
		return "", fmt.Errorf("existing compose .env is invalid: %w", err)
	}
	begin, end, afterEnd := -1, -1, -1
	offset := 0
	for line := range strings.SplitAfterSeq(existing, "\n") {
		text := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		lineOffset := offset
		if offset == 0 && strings.HasPrefix(text, "\ufeff") {
			text = strings.TrimPrefix(text, "\ufeff")
			lineOffset += len("\ufeff")
		}
		switch text {
		case composeEnvBegin:
			if begin >= 0 {
				return "", fmt.Errorf("compose .env contains multiple devhelper managed blocks")
			}
			begin = lineOffset
		case composeEnvEnd:
			if end >= 0 {
				return "", fmt.Errorf("compose .env contains multiple devhelper managed blocks")
			}
			end, afterEnd = lineOffset, offset+len(line)
		}
		offset += len(line)
	}
	var result string
	if begin < 0 && end < 0 {
		result = existing
		if result != "" && !strings.HasSuffix(result, "\n") {
			result += "\n"
		}
		result += generated
	} else {
		if begin < 0 || end < begin {
			return "", fmt.Errorf("compose .env has an incomplete devhelper managed block")
		}
		// A marker inside a multiline quoted value is user data. Refuse to
		// change it instead of replacing part of the user's value.
		if parse(existing[:begin]) != nil || parse(existing[:end]) != nil {
			return "", fmt.Errorf("compose .env managed marker occurs inside a quoted value")
		}
		result = existing[:begin] + generated + existing[afterEnd:]
	}
	if err := parse(result); err != nil {
		return "", fmt.Errorf("generated compose .env is invalid: %w", err)
	}
	return result, nil
}

// Docker Compose expands dollars even inside double quotes. Its $$ escape
// preserves literal dollars, while quoted values preserve # and whitespace.
// Escape backslashes before encoding quotes/control characters so Windows
// paths and passwords ending in a backslash also round-trip correctly.
func quoteComposeValue(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		`"`, `\"`,
		`$`, `$$`,
		"\n", `\n`,
		"\r", `\r`,
		"\t", `\t`,
		"\a", `\a`,
		"\b", `\b`,
		"\f", `\f`,
		"\v", `\v`,
	)
	return `"` + replacer.Replace(value) + `"`
}
