package store

// schema is the authoritative DDL. INTEGER epoch seconds store all timestamps;
// REAL stores all physical magnitudes. Foreign keys enforce that batches and
// items reference existing recipes/reactors/campaigns. The safety_results
// table is a derived (recomputable) projection keyed 1:1 to batches.
const schema = `
CREATE TABLE IF NOT EXISTS reactors (
	id              TEXT PRIMARY KEY,
	name            TEXT NOT NULL,
	volume          REAL NOT NULL,
	heat_transfer_u REAL NOT NULL,
	heat_transfer_area REAL NOT NULL,
	max_operating_temp REAL NOT NULL,
	max_pressure     REAL NOT NULL DEFAULT 0,
	material         TEXT NOT NULL DEFAULT '',
	status           TEXT NOT NULL DEFAULT 'available',
	created_at       INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS recipes (
	id            TEXT PRIMARY KEY,
	name          TEXT NOT NULL,
	product       TEXT NOT NULL,
	k0            REAL NOT NULL,
	ea            REAL NOT NULL,
	reaction_order INTEGER NOT NULL,
	ca0           REAL NOT NULL,
	delta_h       REAL NOT NULL,
	rho           REAL NOT NULL,
	cp            REAL NOT NULL,
	t0            REAL NOT NULL,
	jacket_temp   REAL NOT NULL,
	thermal_limit REAL NOT NULL,
	min_conversion REAL NOT NULL,
	duration      REAL NOT NULL,
	created_at    INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS cleaning_matrix (
	from_product TEXT NOT NULL,
	to_product   TEXT NOT NULL,
	severity     INTEGER NOT NULL DEFAULT 0,
	PRIMARY KEY (from_product, to_product)
);

CREATE TABLE IF NOT EXISTS campaigns (
	id         TEXT PRIMARY KEY,
	name       TEXT NOT NULL,
	status     TEXT NOT NULL DEFAULT 'draft',
	created_at INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS campaign_items (
	campaign_id TEXT NOT NULL REFERENCES campaigns(id),
	seq         INTEGER NOT NULL,
	recipe_id   TEXT NOT NULL REFERENCES recipes(id),
	reactor_id  TEXT NOT NULL REFERENCES reactors(id),
	batch_count INTEGER NOT NULL DEFAULT 1,
	status      TEXT NOT NULL DEFAULT 'pending',
	PRIMARY KEY (campaign_id, seq)
);

CREATE TABLE IF NOT EXISTS batches (
	id               TEXT PRIMARY KEY,
	campaign_id      TEXT NOT NULL REFERENCES campaigns(id),
	campaign_item_seq INTEGER NOT NULL,
	reactor_id       TEXT NOT NULL REFERENCES reactors(id),
	recipe_id        TEXT NOT NULL REFERENCES recipes(id),
	seq              INTEGER NOT NULL,
	status           TEXT NOT NULL DEFAULT 'queued',
	planned_start    INTEGER NOT NULL DEFAULT 0,
	started_at       INTEGER NOT NULL DEFAULT 0,
	ended_at         INTEGER NOT NULL DEFAULT 0,
	peak_temp        REAL NOT NULL DEFAULT 0,
	conversion       REAL NOT NULL DEFAULT 0,
	safety_verdict   TEXT NOT NULL DEFAULT '',
	stoessel_class   INTEGER NOT NULL DEFAULT 0,
	fault_reason     TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_batches_reactor ON batches(reactor_id, seq);
CREATE INDEX IF NOT EXISTS idx_batches_campaign ON batches(campaign_id);

CREATE TABLE IF NOT EXISTS batch_events (
	id         INTEGER PRIMARY KEY AUTOINCREMENT,
	batch_id   TEXT NOT NULL REFERENCES batches(id),
	event_type TEXT NOT NULL,
	at         INTEGER NOT NULL,
	payload    TEXT NOT NULL DEFAULT ''
);

CREATE INDEX IF NOT EXISTS idx_batch_events_batch ON batch_events(batch_id, at);

CREATE TABLE IF NOT EXISTS safety_results (
	batch_id       TEXT PRIMARY KEY REFERENCES batches(id),
	delta_t_ad     REAL NOT NULL,
	mtsr           REAL NOT NULL,
	stoessel_class INTEGER NOT NULL,
	tmr_seconds    REAL NOT NULL,
	peak_temp      REAL NOT NULL,
	conversion     REAL NOT NULL,
	verdict        TEXT NOT NULL
);
`
