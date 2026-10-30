package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
)

var ErrConflict = errors.New("数据已被其他操作修改，请刷新后重试")

type Store struct {
	persist  func([]byte) error
	poisoned bool
	db       *sql.DB
	mu       sync.Mutex
}
type Audit struct {
	ID       int64           `json:"id"`
	Time     string          `json:"time"`
	Action   string          `json:"action"`
	Summary  string          `json:"summary"`
	Revision int             `json:"revision"`
	Before   json.RawMessage `json:"before,omitempty"`
	After    json.RawMessage `json:"after,omitempty"`
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	fail := func(err error) (*Store, error) { _ = db.Close(); return nil, err }
	schema := `PRAGMA foreign_keys=ON; PRAGMA busy_timeout=5000; PRAGMA journal_mode=DELETE; PRAGMA synchronous=FULL;
 CREATE TABLE IF NOT EXISTS meta(id INTEGER PRIMARY KEY CHECK(id=1),schema_version INTEGER NOT NULL,revision INTEGER NOT NULL);
 CREATE TABLE IF NOT EXISTS templates(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS projects(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS sites(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS modules(id TEXT PRIMARY KEY,site_id TEXT NOT NULL REFERENCES sites(id),payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS racks(id TEXT PRIMARY KEY,module_id TEXT NOT NULL REFERENCES modules(id),payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS infrastructure(id TEXT PRIMARY KEY,module_id TEXT NOT NULL REFERENCES modules(id),payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS models(id TEXT PRIMARY KEY,payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS devices(id TEXT PRIMARY KEY,model_id TEXT NOT NULL REFERENCES models(id),module_id TEXT NOT NULL REFERENCES modules(id),rack_id TEXT REFERENCES racks(id),payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS networks(id TEXT PRIMARY KEY,project_id TEXT NOT NULL REFERENCES projects(id),payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS links(id TEXT PRIMARY KEY,project_id TEXT NOT NULL REFERENCES projects(id),source_id TEXT NOT NULL REFERENCES devices(id),target_id TEXT NOT NULL REFERENCES devices(id),payload TEXT NOT NULL);
 CREATE TABLE IF NOT EXISTS audits(id INTEGER PRIMARY KEY AUTOINCREMENT,time TEXT NOT NULL,action TEXT NOT NULL,summary TEXT NOT NULL,revision INTEGER NOT NULL,before_json TEXT NOT NULL,after_json TEXT NOT NULL);
 CREATE INDEX IF NOT EXISTS devices_rack ON devices(rack_id);`
	if _, err = db.Exec(schema); err != nil {
		return fail(err)
	}
	var version int
	err = db.QueryRow("SELECT schema_version FROM meta WHERE id=1").Scan(&version)
	if err == sql.ErrNoRows {
		state := Seed()
		state.Revision = 1
		tx, e := db.Begin()
		if e != nil {
			return fail(e)
		}
		defer tx.Rollback()
		if e = saveState(tx, state); e != nil {
			return fail(e)
		}
		after, _ := json.Marshal(state)
		if e = insertBusinessAudit(tx, "initialize", "初始化17柜规划与型号库（未导入真实设备）", 1, json.RawMessage("null"), after); e != nil {
			return fail(e)
		}
		if e = tx.Commit(); e != nil {
			return fail(e)
		}
	} else if err != nil {
		return fail(err)
	} else if version >= 1 && version <= 4 {
		if err = s.migrateHierarchy(path); err != nil {
			return fail(err)
		}
	} else if version != 5 {
		return fail(fmt.Errorf("数据库版本%d不受支持，请使用对应版本程序", version))
	}
	return s, nil
}
func (s *Store) Close() error { return s.db.Close() }

type queryer interface {
	Query(string, ...any) (*sql.Rows, error)
	QueryRow(string, ...any) *sql.Row
}

func loadRows[T any](q queryer, table string) ([]T, error) {
	rows, err := q.Query("SELECT payload FROM " + table + " ORDER BY rowid")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []T{}
	for rows.Next() {
		var b []byte
		if err = rows.Scan(&b); err != nil {
			return nil, err
		}
		var v T
		if err = json.Unmarshal(b, &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
func loadState(q queryer) (State, error) {
	s := State{}
	if err := q.QueryRow("SELECT schema_version,revision FROM meta WHERE id=1").Scan(&s.SchemaVersion, &s.Revision); err != nil {
		return s, err
	}
	var err error
	if s.Templates, err = loadRows[ModuleTemplate](q, "templates"); err != nil {
		return s, err
	}
	if s.Networks, err = loadRows[Network](q, "networks"); err != nil {
		return s, err
	}
	if s.Links, err = loadRows[Link](q, "links"); err != nil {
		return s, err
	}
	if s.Projects, err = loadRows[Project](q, "projects"); err != nil {
		return s, err
	}
	if s.Sites, err = loadRows[Site](q, "sites"); err != nil {
		return s, err
	}
	if s.Modules, err = loadRows[Module](q, "modules"); err != nil {
		return s, err
	}
	if s.Racks, err = loadRows[Rack](q, "racks"); err != nil {
		return s, err
	}
	if s.Infra, err = loadRows[Infra](q, "infrastructure"); err != nil {
		return s, err
	}
	if s.Models, err = loadRows[Model](q, "models"); err != nil {
		return s, err
	}
	s.Devices, err = loadRows[Device](q, "devices")
	return s, err
}
func saveState(tx *sql.Tx, s State) error {
	for _, table := range []string{"templates", "links", "networks", "devices", "infrastructure", "racks", "modules", "sites", "models", "projects"} {
		if _, err := tx.Exec("DELETE FROM " + table); err != nil {
			return err
		}
	}
	put := func(query string, v any, args ...any) error {
		b, err := json.Marshal(v)
		if err != nil {
			return err
		}
		args = append(args, string(b))
		_, err = tx.Exec(query, args...)
		return err
	}
	for _, v := range s.Templates {
		if err := put("INSERT INTO templates(id,payload) VALUES(?,?)", v, v.ID); err != nil {
			return err
		}
	}
	for _, v := range s.Projects {
		if err := put("INSERT INTO projects(id,payload) VALUES(?,?)", v, v.ID); err != nil {
			return err
		}
	}
	for _, v := range s.Sites {
		if err := put("INSERT INTO sites(id,payload) VALUES(?,?)", v, v.ID); err != nil {
			return err
		}
	}
	for _, v := range s.Models {
		if err := put("INSERT INTO models(id,payload) VALUES(?,?)", v, v.ID); err != nil {
			return err
		}
	}
	for _, v := range s.Modules {
		if err := put("INSERT INTO modules(id,site_id,payload) VALUES(?,?,?)", v, v.ID, v.SiteID); err != nil {
			return err
		}
	}
	for _, v := range s.Racks {
		if err := put("INSERT INTO racks(id,module_id,payload) VALUES(?,?,?)", v, v.ID, v.ModuleID); err != nil {
			return err
		}
	}
	for _, v := range s.Infra {
		if err := put("INSERT INTO infrastructure(id,module_id,payload) VALUES(?,?,?)", v, v.ID, v.ModuleID); err != nil {
			return err
		}
	}
	for _, v := range s.Devices {
		var rack any
		if v.RackID != "" {
			rack = v.RackID
		}
		if err := put("INSERT INTO devices(id,model_id,module_id,rack_id,payload) VALUES(?,?,?,?,?)", v, v.ID, v.ModelID, v.ModuleID, rack); err != nil {
			return err
		}
	}
	for _, v := range s.Networks {
		if err := put("INSERT INTO networks(id,project_id,payload) VALUES(?,?,?)", v, v.ID, v.ProjectID); err != nil {
			return err
		}
	}
	for _, v := range s.Links {
		if err := put("INSERT INTO links(id,project_id,source_id,target_id,payload) VALUES(?,?,?,?,?)", v, v.ID, v.ProjectID, v.SourceID, v.TargetID); err != nil {
			return err
		}
	}
	_, err := tx.Exec("INSERT INTO meta(id,schema_version,revision) VALUES(1,?,?) ON CONFLICT(id) DO UPDATE SET schema_version=excluded.schema_version,revision=excluded.revision", s.SchemaVersion, s.Revision)
	return err
}
func (s *Store) Snapshot() (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return State{}, ErrPersistence
	}
	tx, err := s.db.Begin()
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback()
	state, err := loadState(tx)
	if err != nil {
		return state, err
	}
	return state, tx.Commit()
}
func (s *Store) Apply(revision int, action string, fn func(*State) (string, error)) (State, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return State{}, ErrPersistence
	}
	conn, err := s.db.Conn(context.Background())
	if err != nil {
		return State{}, err
	}
	defer conn.Close()
	tx, err := conn.BeginTx(context.Background(), nil)
	if err != nil {
		return State{}, err
	}
	defer tx.Rollback()
	old, err := loadState(tx)
	if err != nil {
		return State{}, err
	}
	if revision != old.Revision {
		return State{}, ErrConflict
	}
	next := Clone(old)
	summary, err := fn(&next)
	if err != nil {
		return State{}, err
	}
	if err = Validate(&next); err != nil {
		return State{}, err
	}
	next.Revision++
	if err = saveState(tx, next); err != nil {
		return State{}, err
	}
	before, _ := json.Marshal(old)
	after, _ := json.Marshal(next)
	if err = insertBusinessAudit(tx, action, summary, next.Revision, before, after); err != nil {
		return State{}, err
	}
	if s.persist != nil {
		image, e := exportImage(tx)
		if e != nil {
			return State{}, ErrPersistence
		}
		defer clear(image)
		if e = s.persist(image); e != nil {
			return State{}, ErrPersistence
		}
	}
	if err = tx.Commit(); err != nil {
		if s.persist != nil {
			s.poisoned = true
			return State{}, ErrPersistence
		}
		return State{}, err
	}
	return next, nil
}
func (s *Store) Audits() ([]Audit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return nil, ErrPersistence
	}
	rows, err := s.db.Query("SELECT id,time,action,summary,revision FROM audits ORDER BY id DESC LIMIT 200")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Audit{}
	for rows.Next() {
		var a Audit
		if err = rows.Scan(&a.ID, &a.Time, &a.Action, &a.Summary, &a.Revision); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
}
func (s *Store) Audit(id int64) (Audit, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return Audit{}, ErrPersistence
	}
	var a Audit
	var before, after string
	err := s.db.QueryRow("SELECT id,time,action,summary,revision,before_json,after_json FROM audits WHERE id=?", id).Scan(&a.ID, &a.Time, &a.Action, &a.Summary, &a.Revision, &before, &after)
	a.Before = json.RawMessage(before)
	a.After = json.RawMessage(after)
	return a, err
}
