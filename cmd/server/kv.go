package main

import (
	"database/sql"
	_ "github.com/lib/pq"
)

// kvStore is a wrapper for a KV store backed by PostgreSQL.
type kvStore struct {
	db *sql.DB
}

// newKVStore initializes a new kvStore with a PostgreSQL database connection.
func newKVStore(db *sql.DB) (*kvStore, error) {
	kv := &kvStore{db: db}

	// Create the table if it doesn't exist
	query := `
	CREATE TABLE IF NOT EXISTS kv_store (
		id TEXT NOT NULL,
		value BYTEA,
		PRIMARY KEY (id)
	) PARTITION BY HASH (id);

	CREATE TABLE IF NOT EXISTS kv_store_p0 PARTITION OF kv_store FOR VALUES WITH (MODULUS 4, REMAINDER 0);
	CREATE TABLE IF NOT EXISTS kv_store_p1 PARTITION OF kv_store FOR VALUES WITH (MODULUS 4, REMAINDER 1);
	CREATE TABLE IF NOT EXISTS kv_store_p2 PARTITION OF kv_store FOR VALUES WITH (MODULUS 4, REMAINDER 2);
	CREATE TABLE IF NOT EXISTS kv_store_p3 PARTITION OF kv_store FOR VALUES WITH (MODULUS 4, REMAINDER 3);

	CREATE TABLE IF NOT EXISTS kv_store_shadow (
		id TEXT,
		value BYTEA,
		PRIMARY KEY (id, value)
	);
	CREATE INDEX IF NOT EXISTS kv_store_shadow_values ON kv_store_shadow (value);
	`
	_, err := db.Exec(query)
	if err != nil {
		return nil, err
	}

	return kv, nil
}

// Put a value at key id and replace any existing value.
func (kv *kvStore) Put(id string, value []byte) error {
	query := `
	INSERT INTO kv_store (id, value) VALUES ($1, $2)
	ON CONFLICT (id) DO UPDATE SET value = $2;`
	_, err := kv.db.Exec(query, id, value)
	return err
}

// Put a value at key id and replace any existing value.
func (kv *kvStore) insertShadow(id string, value []byte) error {
	query := `
	INSERT INTO kv_store_shadow (id, value) VALUES ($1, $2)
	ON CONFLICT (id, value) DO NOTHING;`
	_, err := kv.db.Exec(query, id, value)
	return err
}

// Append a value to any existing value at key id.
func (kv *kvStore) Append(id string, value []byte) error {
	query := `SELECT value FROM kv_store WHERE id = $1`
	var existingValue []byte
	err := kv.db.QueryRow(query, id).Scan(&existingValue)
	if err != nil {
		if err == sql.ErrNoRows {
			return kv.Put(id, value)
		}
		return err
	}

	newValue := append(existingValue, value...)
	return kv.Put(id, newValue)
}

// Get returns the value in the key identified by id.
func (kv *kvStore) Get(id string) ([]byte, error) {
	query := `SELECT value FROM kv_store WHERE id = $1`
	var value []byte
	err := kv.db.QueryRow(query, id).Scan(&value)
	if err != nil {
		if err == sql.ErrNoRows {
			return []byte{}, nil
		}
		return nil, err
	}
	return value, nil
}

// checkIfUnique checks if the value for a given id is unique in the shadow table.
func (kv *kvStore) checkIfUnique(value []byte) bool {
	query := `SELECT 1 FROM kv_store_shadow WHERE value = $1`
	var exists int
	err := kv.db.QueryRow(query, value).Scan(&exists)
	return err == sql.ErrNoRows
}
