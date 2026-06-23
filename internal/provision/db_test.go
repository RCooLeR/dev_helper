package provision

import "testing"

func TestNormalizeDBEngine(t *testing.T) {
	tests := map[string]string{
		"mysql":        "mysql-8.4",
		"mysql-8.4":    "mysql-8.4",
		"mysql9":       "mysql-9.7",
		"mysql-9.6":    "mysql-9.7",
		"mysql-9.7":    "mysql-9.7",
		"mariadb-10.6": "mariadb10",
		"postgresql":   "postgres",
		"none":         "none",
	}

	for in, want := range tests {
		if got := normalizeDBEngine(in); got != want {
			t.Fatalf("normalizeDBEngine(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestSQLQuotingHelpers(t *testing.T) {
	if got := mysqlIdent("a`b"); got != "`a``b`" {
		t.Fatalf("mysqlIdent escaped incorrectly: %q", got)
	}
	if got := mysqlString("a'b"); got != "'a''b'" {
		t.Fatalf("mysqlString escaped incorrectly: %q", got)
	}
	if got := pgIdent(`a"b`); got != `"a""b"` {
		t.Fatalf("pgIdent escaped incorrectly: %q", got)
	}
	if got := pgString("a'b"); got != "'a''b'" {
		t.Fatalf("pgString escaped incorrectly: %q", got)
	}
}
