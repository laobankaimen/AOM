package core

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"
)

const auditDeltaFormat = "changed-records-v1"

var auditCollections = []string{"projects", "sites", "modules", "racks", "infra", "models", "devices", "networks", "links", "templates"}
var visualFields = map[string][]string{"modules": {"roomPosition", "roomRotation"}, "devices": {"topologyPosition"}, "networks": {"position", "devicePositions", "color"}, "templates": {"updatedAt"}}

func auditObjects(raw json.RawMessage, collection string) (map[string]json.RawMessage, error) {
	var records []map[string]json.RawMessage
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, &records); err != nil {
			return nil, err
		}
	}
	result := map[string]json.RawMessage{}
	for _, record := range records {
		var id string
		if err := json.Unmarshal(record["id"], &id); err != nil || id == "" {
			return nil, fmt.Errorf("审计对象缺少编号")
		}
		if _, exists := result[id]; exists {
			return nil, fmt.Errorf("审计对象编号重复")
		}
		for _, field := range visualFields[collection] {
			delete(record, field)
		}
		data, err := json.Marshal(record)
		if err != nil {
			return nil, err
		}
		result[id] = data
	}
	return result, nil
}

// Keep only changed business objects, including the old values of deleted ones.
// Never strip an unfamiliar historical format: callers leave that entry intact.
func auditDelta(before, after json.RawMessage) (json.RawMessage, json.RawMessage, bool, error) {
	var a, b map[string]json.RawMessage
	if err := json.Unmarshal(before, &a); err != nil {
		return nil, nil, false, err
	}
	if err := json.Unmarshal(after, &b); err != nil {
		return nil, nil, false, err
	}
	if a == nil {
		a = map[string]json.RawMessage{}
	}
	if b == nil {
		b = map[string]json.RawMessage{}
	}
	if len(a["format"]) > 0 || len(b["format"]) > 0 {
		return nil, nil, false, fmt.Errorf("已有精简格式或未知格式")
	}
	if len(b["schemaVersion"]) == 0 {
		return nil, nil, false, fmt.Errorf("不是完整状态快照")
	}
	allowed := map[string]bool{"revision": true, "schemaVersion": true}
	for _, key := range auditCollections {
		allowed[key] = true
	}
	for _, snapshot := range []map[string]json.RawMessage{a, b} {
		for key := range snapshot {
			if !allowed[key] {
				return nil, nil, false, fmt.Errorf("未知审计字段：%s", key)
			}
		}
	}
	old, new := map[string]any{"format": auditDeltaFormat}, map[string]any{"format": auditDeltaFormat}
	if len(a["revision"]) > 0 {
		old["revision"] = a["revision"]
	}
	if len(b["revision"]) > 0 {
		new["revision"] = b["revision"]
	}
	changed := false
	for _, key := range auditCollections {
		left, err := auditObjects(a[key], key)
		if err != nil {
			return nil, nil, false, err
		}
		right, err := auditObjects(b[key], key)
		if err != nil {
			return nil, nil, false, err
		}
		// Preserve original snapshot order for readable batch imports.
		oldList, newList := []json.RawMessage{}, []json.RawMessage{}
		for _, side := range []struct {
			raw     json.RawMessage
			objects map[string]json.RawMessage
			out     *[]json.RawMessage
		}{{a[key], left, &oldList}, {b[key], right, &newList}} {
			var records []struct {
				ID string `json:"id"`
			}
			if len(side.raw) > 0 {
				if err = json.Unmarshal(side.raw, &records); err != nil {
					return nil, nil, false, err
				}
			}
			for _, record := range records {
				if !bytes.Equal(left[record.ID], right[record.ID]) {
					*side.out = append(*side.out, side.objects[record.ID])
				}
			}
		}
		if len(oldList)+len(newList) > 0 {
			changed = true
			old[key] = oldList
			new[key] = newList
		}
	}
	beforeJSON, err := json.Marshal(old)
	if err != nil {
		return nil, nil, false, err
	}
	afterJSON, err := json.Marshal(new)
	return beforeJSON, afterJSON, changed, err
}
func insertBusinessAudit(tx *sql.Tx, action, summary string, revision int, before, after json.RawMessage) error {
	old, new, changed, err := auditDelta(before, after)
	if err != nil {
		return err
	}
	if !changed {
		return nil
	}
	_, err = tx.Exec("INSERT INTO audits(time,action,summary,revision,before_json,after_json) VALUES(?,?,?,?,?,?)", time.Now().UTC().Format(time.RFC3339), action, summary, revision, string(old), string(new))
	return err
}

type AuditCompaction struct {
	RecordsBefore int    `json:"recordsBefore"`
	RecordsAfter  int    `json:"recordsAfter"`
	Removed       int    `json:"removed"`
	Compacted     int    `json:"compacted"`
	BytesBefore   int64  `json:"bytesBefore"`
	BytesAfter    int64  `json:"bytesAfter"`
	Warning       string `json:"warning,omitempty"`
}

func (s *Store) CompactAudits(revision int) (AuditCompaction, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	result := AuditCompaction{}
	if s.poisoned {
		return result, ErrPersistence
	}
	tx, err := s.db.Begin()
	if err != nil {
		return result, err
	}
	defer tx.Rollback()
	var current int
	if err = tx.QueryRow("SELECT revision FROM meta WHERE id=1").Scan(&current); err != nil {
		return result, err
	}
	if current != revision {
		return result, ErrConflict
	}
	rows, err := tx.Query("SELECT id,before_json,after_json FROM audits ORDER BY id")
	if err != nil {
		return result, err
	}
	type row struct {
		id            int64
		before, after string
	}
	entries := []row{}
	for rows.Next() {
		var v row
		if err = rows.Scan(&v.id, &v.before, &v.after); err != nil {
			rows.Close()
			return result, err
		}
		entries = append(entries, v)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return result, err
	}
	result.RecordsBefore = len(entries)
	for _, entry := range entries {
		result.BytesBefore += int64(len(entry.before) + len(entry.after))
		result.RecordsAfter++
		old, new, changed, e := auditDelta(json.RawMessage(entry.before), json.RawMessage(entry.after))
		if e != nil {
			result.BytesAfter += int64(len(entry.before) + len(entry.after))
			continue
		}
		if !changed {
			if _, err = tx.Exec("DELETE FROM audits WHERE id=?", entry.id); err != nil {
				return result, err
			}
			result.Removed++
			result.RecordsAfter--
			continue
		}
		if !bytes.Equal(old, []byte(entry.before)) || !bytes.Equal(new, []byte(entry.after)) {
			if _, err = tx.Exec("UPDATE audits SET before_json=?,after_json=? WHERE id=?", string(old), string(new), entry.id); err != nil {
				return result, err
			}
			result.Compacted++
		}
		result.BytesAfter += int64(len(old) + len(new))
	}
	if result.Removed+result.Compacted == 0 {
		return result, tx.Commit()
	}
	if s.persist != nil {
		image, e := exportImage(tx)
		if e != nil {
			return result, ErrPersistence
		}
		defer clear(image)
		if e = s.persist(image); e != nil {
			return result, ErrPersistence
		}
	}
	if err = tx.Commit(); err != nil {
		if s.persist != nil {
			s.poisoned = true
			return result, ErrPersistence
		}
		return result, err
	}
	if s.persist == nil {
		if _, err = s.db.Exec("VACUUM"); err != nil {
			result.Warning = "记录已精简，但文件空间整理失败，可稍后重试"
		}
	}
	return result, nil
}
