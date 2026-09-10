package core

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"

	_ "modernc.org/sqlite"
)

var ErrStaleRevision = errors.New("stale configuration revision")

type store struct{ db *sql.DB }

func openStore(path string) (*store, Config, error) {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, Config{}, err
	}
	if _, err = db.Exec(`CREATE TABLE IF NOT EXISTS configuration (id INTEGER PRIMARY KEY CHECK(id=1), revision INTEGER NOT NULL, document BLOB NOT NULL)`); err != nil {
		db.Close()
		return nil, Config{}, err
	}
	var revision int64
	var raw []byte
	err = db.QueryRow(`SELECT revision, document FROM configuration WHERE id=1`).Scan(&revision, &raw)
	if errors.Is(err, sql.ErrNoRows) {
		return &store{db}, Config{}, nil
	}
	if err != nil {
		db.Close()
		return nil, Config{}, err
	}
	var config Config
	if err := json.Unmarshal(raw, &config); err != nil {
		db.Close()
		return nil, Config{}, fmt.Errorf("decode configuration: %w", err)
	}
	config.Revision = revision
	return &store{db}, config, nil
}

func (s *store) save(config Config, expected int64) (Config, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return Config{}, err
	}
	defer tx.Rollback()
	var current int64
	err = tx.QueryRow(`SELECT revision FROM configuration WHERE id=1`).Scan(&current)
	if errors.Is(err, sql.ErrNoRows) {
		current = 0
	} else if err != nil {
		return Config{}, err
	}
	if current != expected {
		return Config{}, ErrStaleRevision
	}
	config.Revision = current + 1
	raw, err := json.Marshal(config)
	if err != nil {
		return Config{}, err
	}
	if _, err = tx.Exec(`INSERT INTO configuration(id,revision,document) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET revision=excluded.revision, document=excluded.document`, config.Revision, raw); err != nil {
		return Config{}, err
	}
	if err = tx.Commit(); err != nil {
		return Config{}, err
	}
	return config, nil
}
