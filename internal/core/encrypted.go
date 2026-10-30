package core

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path/filepath"
	"strings"
)

var ErrPersistence = errors.New("加密保存失败，操作未完成，请检查磁盘空间和目录权限")

type encryptedImage struct {
	Format string  `json:"format"`
	State  State   `json:"state"`
	Audits []Audit `json:"audits"`
}

// SQLite stays entirely in memory. The persistent encrypted representation
// contains the complete state and full audit history, with stable record IDs.
func OpenMemory(image []byte, persist func([]byte) error) (*Store, error) {
	s, e := Open(":memory:")
	if e != nil {
		return nil, e
	}
	fail := func(e error) (*Store, error) { s.Close(); return nil, e }
	if _, e = s.db.Exec("PRAGMA temp_store=MEMORY; PRAGMA journal_mode=MEMORY;"); e != nil {
		return fail(e)
	}
	if len(image) > 0 {
		var content encryptedImage
		if e = json.Unmarshal(image, &content); e != nil || content.Format != "aom-encrypted-state-1" {
			return fail(errors.New("加密数据内容无效"))
		}
		if e = Validate(&content.State); e != nil {
			return fail(fmt.Errorf("数据内容无法加载：%w", e))
		}
		e = func() error {
			tx, e := s.db.Begin()
			if e != nil {
				return e
			}
			defer tx.Rollback()
			if e = saveState(tx, content.State); e != nil {
				return e
			}
			if _, e = tx.Exec("DELETE FROM audits"); e != nil {
				return e
			}
			for _, a := range content.Audits {
				if a.ID < 1 || !json.Valid(a.Before) || !json.Valid(a.After) {
					return errors.New("审计快照无效")
				}
				if _, e = tx.Exec("INSERT INTO audits(id,time,action,summary,revision,before_json,after_json) VALUES(?,?,?,?,?,?,?)", a.ID, a.Time, a.Action, a.Summary, a.Revision, string(a.Before), string(a.After)); e != nil {
					return e
				}
			}
			return tx.Commit()
		}()
		if e != nil {
			return fail(e)
		}
	}
	s.persist = persist
	return s, nil
}
func exportImage(q queryer) ([]byte, error) {
	state, e := loadState(q)
	if e != nil {
		return nil, e
	}
	rows, e := q.Query("SELECT id,time,action,summary,revision,before_json,after_json FROM audits ORDER BY id")
	if e != nil {
		return nil, e
	}
	defer rows.Close()
	audits := []Audit{}
	for rows.Next() {
		var a Audit
		var before, after string
		if e = rows.Scan(&a.ID, &a.Time, &a.Action, &a.Summary, &a.Revision, &before, &after); e != nil {
			return nil, e
		}
		a.Before = json.RawMessage(before)
		a.After = json.RawMessage(after)
		audits = append(audits, a)
	}
	if e = rows.Err(); e != nil {
		return nil, e
	}
	return json.Marshal(encryptedImage{Format: "aom-encrypted-state-1", State: state, Audits: audits})
}
func (s *Store) ExportImage() ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.poisoned {
		return nil, ErrPersistence
	}
	tx, e := s.db.Begin()
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	return exportImage(tx)
}

// Reads old SQLite without writing, deleting or changing its source file.
func ReadPlaintextImage(path string) ([]byte, error) {
	abs, e := filepath.Abs(path)
	if e != nil {
		return nil, e
	}
	uriPath := filepath.ToSlash(abs)
	if filepath.VolumeName(abs) != "" && !strings.HasPrefix(uriPath, "/") {
		uriPath = "/" + uriPath
	}
	u := url.URL{Scheme: "file", Path: uriPath}
	q := u.Query()
	q.Set("mode", "ro")
	u.RawQuery = q.Encode()
	db, e := sql.Open("sqlite", u.String())
	if e != nil {
		return nil, e
	}
	defer db.Close()
	db.SetMaxOpenConns(1)
	conn, e := db.Conn(context.Background())
	if e != nil {
		return nil, e
	}
	defer conn.Close()
	tx, e := conn.BeginTx(context.Background(), &sql.TxOptions{ReadOnly: true})
	if e != nil {
		return nil, e
	}
	defer tx.Rollback()
	var schema int
	if e = tx.QueryRow("SELECT schema_version FROM meta WHERE id=1").Scan(&schema); e != nil {
		return nil, e
	}
	if schema != 5 {
		return nil, errors.New("现有数据库结构不受支持，请先使用匹配的应用导出数据")
	}
	return exportImage(tx)
}
