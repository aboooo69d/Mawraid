package database

import (
	"testing"
)

// TestEmbeddedMigrationsDiscovered guards the ".up.sql" file filter against the
// original off-by-three defect (the runner sliced the last 11 characters and
// compared them to the 8-character ".up.sql" suffix, so no migration ever
// matched and Migrate silently applied nothing). It runs the real upMigrations
// helper over the embedded FS and needs no database connection.
func TestEmbeddedMigrationsDiscovered(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatalf("read embedded migrations: %v", err)
	}

	files := upMigrations(entries)
	if len(files) == 0 {
		t.Fatal("no .up.sql migrations discovered in embedded FS")
	}
	if !contains(files, "0001_init.up.sql") {
		t.Fatalf("expected 0001_init.up.sql to be discovered, got %v", files)
	}
}

func contains(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
