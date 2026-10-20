package state

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/golang-migrate/migrate/v4"
	migratesqlite "github.com/golang-migrate/migrate/v4/database/sqlite"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// migrateToVersion steps a database up to an explicit version, which is what a
// real 12 -> 13 upgrade looks like on an existing state.db.
func migrateToVersion(t *testing.T, db *sql.DB, version int) {
	t.Helper()
	src, err := iofs.New(migrationsFS, stateMigrationsPath)
	if err != nil {
		t.Fatalf("iofs: %v", err)
	}
	drv, err := migratesqlite.WithInstance(db, &migratesqlite.Config{MigrationsTable: migrateDefaultTable})
	if err != nil {
		t.Fatalf("driver: %v", err)
	}
	m, err := migrate.NewWithInstance("iofs", src, "sqlite", drv)
	if err != nil {
		t.Fatalf("migrator: %v", err)
	}
	if err := m.Migrate(uint(version)); err != nil && err != migrate.ErrNoChange {
		t.Fatalf("migrate to %d: %v", version, err)
	}
}

func currentVersion(t *testing.T, db *sql.DB) int {
	t.Helper()
	var v int
	var dirty bool
	if err := db.QueryRow(`SELECT version, dirty FROM schema_migrations`).Scan(&v, &dirty); err != nil {
		t.Fatalf("read version: %v", err)
	}
	if dirty {
		t.Fatal("migration marked dirty")
	}
	return v
}

// A pre-policy platform must survive 12 -> 13 with the new columns defaulted.
func TestMigrateStateDB_UpgradeFrom12KeepsOldPlatform(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	migrateToVersion(t, db, stateVersionDropBlacklistGrace)
	if got := currentVersion(t, db); got != 12 {
		t.Fatalf("version after step 1 = %d, want 12", got)
	}

	// Insert an old-shape platform: only the columns that existed at v12.
	_, err = db.Exec(`
		INSERT INTO platforms (
			id, name, sticky_ttl_ns, regex_filters_json, region_filters_json,
			reverse_proxy_miss_action, reverse_proxy_empty_account_behavior,
			reverse_proxy_fixed_account_header, allocation_policy,
			passive_circuit_breaker_disabled, updated_at_ns
		) VALUES (
			'plat-legacy', 'Legacy', 3600000000000, '["^agg/"]', '["us"]',
			'TREAT_AS_EMPTY', 'RANDOM', 'Authorization', 'BALANCED', 0, 1700000000000000000
		)`)
	if err != nil {
		t.Fatalf("insert legacy platform: %v", err)
	}

	if err := MigrateStateDB(db); err != nil {
		t.Fatalf("migrate 12 -> 13: %v", err)
	}
	if got := currentVersion(t, db); got != stateLatestVersion {
		t.Fatalf("version = %d, want %d", got, stateLatestVersion)
	}

	var (
		name             string
		regexJSON        string
		ic               int
		failoverEnabled  int
		maxSwitches      int
		deadline         int64
		stage            string
		probeEnabled     int
		verdictEnabled   int
	)
	err = db.QueryRow(`
		SELECT name, regex_filters_json,
		       passive_circuit_breaker_disabled,
		       failover_enabled, failover_max_switches, failover_deadline_ns, failover_stage,
		       probe_enabled, verdict_enabled
		FROM platforms WHERE id = 'plat-legacy'`).Scan(
		&name, &regexJSON, &ic, &failoverEnabled, &maxSwitches, &deadline, &stage,
		&probeEnabled, &verdictEnabled,
	)
	if err != nil {
		t.Fatalf("read legacy platform: %v", err)
	}

	// Pre-existing values untouched.
	if name != "Legacy" {
		t.Fatalf("name = %q, want Legacy", name)
	}
	if regexJSON != `["^agg/"]` {
		t.Fatalf("regex_filters_json = %q", regexJSON)
	}
	if ic != 0 {
		t.Fatalf("passive_circuit_breaker_disabled = %d, want 0", ic)
	}

	// New columns at their documented defaults.
	if probeEnabled != 0 {
		t.Fatalf("probe_enabled = %d, want 0 (probe off by default)", probeEnabled)
	}
	if verdictEnabled != 0 {
		t.Fatalf("verdict_enabled = %d, want 0", verdictEnabled)
	}
	if failoverEnabled != 0 {
		t.Fatalf("failover_enabled = %d, want 0", failoverEnabled)
	}
	if maxSwitches != 3 {
		t.Fatalf("failover_max_switches = %d, want 3", maxSwitches)
	}
	if deadline != 15000000000 {
		t.Fatalf("failover_deadline_ns = %d, want 15000000000 (15s)", deadline)
	}
	if stage != "connect" {
		t.Fatalf("failover_stage = %q, want connect", stage)
	}
}

// The upgraded platform must round-trip through the repo layer unchanged.
func TestMigrateStateDB_UpgradeFrom12RepoReadsLegacyDefaults(t *testing.T) {
	path := filepath.Join(t.TempDir(), "state.db")
	db, err := OpenDB(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer db.Close()

	migrateToVersion(t, db, stateVersionDropBlacklistGrace)
	if _, err := db.Exec(`
		INSERT INTO platforms (id, name, sticky_ttl_ns, regex_filters_json, region_filters_json,
			reverse_proxy_miss_action, reverse_proxy_empty_account_behavior,
			reverse_proxy_fixed_account_header, allocation_policy,
			passive_circuit_breaker_disabled, updated_at_ns)
		VALUES ('p2', 'OldPlatform', 3600000000000, '[]', '[]',
			'TREAT_AS_EMPTY', 'RANDOM', '', 'BALANCED', 0, 1)`); err != nil {
		t.Fatalf("insert: %v", err)
	}
	if err := MigrateStateDB(db); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	repo := newStateRepo(db)
	p, err := repo.GetPlatform("p2")
	if err != nil {
		t.Fatalf("GetPlatform: %v", err)
	}
	if p.ProbeEnabled || p.VerdictEnabled || p.FailoverEnabled {
		t.Fatalf("all policy toggles must default to false, got probe=%v verdict=%v failover=%v",
			p.ProbeEnabled, p.VerdictEnabled, p.FailoverEnabled)
	}
	if p.FailoverMaxSwitches != 3 || p.FailoverDeadlineNs != 15000000000 || p.FailoverStage != "connect" {
		t.Fatalf("failover defaults wrong: %d %d %q", p.FailoverMaxSwitches, p.FailoverDeadlineNs, p.FailoverStage)
	}
	if p.LatencyTopN != 0 || p.LatencyMaxMs != 0 || len(p.EgressIPFilters) != 0 {
		t.Fatalf("P1 policy defaults must be zero/empty")
	}
}
