package storage

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/79E/auto-openai-account/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct{ db *sql.DB }

func Open(path string) (*Store, error) {
	if path == "" {
		path = filepath.Join("data", "register.db")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	s := &Store{db: db}
	if err := s.init(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	if s == nil || s.db == nil {
		return nil
	}
	return s.db.Close()
}

func now() string        { return time.Now().UTC().Format(time.RFC3339Nano) }
func clean(v any) string { return strings.TrimSpace(fmt.Sprint(v)) }

func (s *Store) init() error {
	stmts := []string{
		`PRAGMA journal_mode=WAL`, `PRAGMA synchronous=NORMAL`, `PRAGMA busy_timeout=5000`,
		`CREATE TABLE IF NOT EXISTS settings (key TEXT PRIMARY KEY, value TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS mailboxes (id INTEGER PRIMARY KEY AUTOINCREMENT, email TEXT NOT NULL UNIQUE, password TEXT NOT NULL, client_id TEXT, access_token TEXT, status TEXT NOT NULL DEFAULT 'new', register_password TEXT, token_json TEXT, remark TEXT, last_error TEXT, current_step TEXT, current_step_index INTEGER NOT NULL DEFAULT 0, current_step_total INTEGER NOT NULL DEFAULT 0, proxy TEXT, registered_at TEXT, last_login_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_mailboxes_status_id ON mailboxes (status, id)`,
		`CREATE TABLE IF NOT EXISTS register_jobs (id INTEGER PRIMARY KEY AUTOINCREMENT, status TEXT NOT NULL, requested_count INTEGER NOT NULL, total_count INTEGER NOT NULL, success_count INTEGER NOT NULL DEFAULT 0, failed_count INTEGER NOT NULL DEFAULT 0, success_rate REAL NOT NULL DEFAULT 0, avg_duration_ms INTEGER NOT NULL DEFAULT 0, total_duration_ms INTEGER NOT NULL DEFAULT 0, started_at TEXT, finished_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`ALTER TABLE register_jobs ADD COLUMN type TEXT NOT NULL DEFAULT 'register'`,
		`CREATE TABLE IF NOT EXISTS register_job_items (id INTEGER PRIMARY KEY AUTOINCREMENT, job_id INTEGER NOT NULL, mailbox_id INTEGER NOT NULL, email TEXT NOT NULL, status TEXT NOT NULL, error TEXT, duration_ms INTEGER NOT NULL DEFAULT 0, started_at TEXT, finished_at TEXT, created_at TEXT NOT NULL, updated_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_register_job_items_job_id ON register_job_items (job_id, id)`,
		`CREATE TABLE IF NOT EXISTS runtime_logs (id INTEGER PRIMARY KEY AUTOINCREMENT, job_id INTEGER NOT NULL DEFAULT 0, mailbox_id INTEGER NOT NULL DEFAULT 0, email TEXT, level TEXT NOT NULL, step TEXT, step_index INTEGER NOT NULL DEFAULT 0, step_total INTEGER NOT NULL DEFAULT 0, message TEXT NOT NULL, created_at TEXT NOT NULL)`,
		`CREATE INDEX IF NOT EXISTS idx_runtime_logs_job_id_id ON runtime_logs (job_id, id)`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil && !strings.Contains(err.Error(), "duplicate column") {
			return err
		}
	}
	var count int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM settings`).Scan(&count); err != nil {
		return err
	}
	if count == 0 {
		return s.SaveSettings(domain.DefaultSettings())
	}
	_, _ = s.db.Exec(`UPDATE settings SET value = '"auto"' WHERE key = 'imap_auth_mode' AND value = '"xoauth2"'`)
	return nil
}

func (s *Store) LoadSettings() (domain.Settings, error) {
	settings := domain.DefaultSettings()
	rows, err := s.db.Query(`SELECT key, value FROM settings`)
	if err != nil {
		return settings, err
	}
	defer rows.Close()
	values := map[string]any{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return settings, err
		}
		var decoded any
		if json.Unmarshal([]byte(v), &decoded) != nil {
			decoded = v
		}
		values[k] = decoded
	}
	data, _ := json.Marshal(values)
	_ = json.Unmarshal(data, &settings)
	return domain.NormalizeSettings(settings), rows.Err()
}

func (s *Store) SaveSettings(settings domain.Settings) error {
	settings = domain.NormalizeSettings(settings)
	data, err := json.Marshal(settings)
	if err != nil {
		return err
	}
	values := map[string]any{}
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()
	ts := now()
	for k, v := range values {
		enc, _ := json.Marshal(v)
		if _, err := tx.Exec(`INSERT INTO settings (key, value, updated_at) VALUES (?, ?, ?) ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`, k, string(enc), ts); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) ImportMailboxes(items []domain.Mailbox) (int, int, []string, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, nil, err
	}
	defer tx.Rollback()
	imported, skipped := 0, 0
	errs := []string{}
	ts := now()
	for _, item := range items {
		email := strings.ToLower(clean(item.Email))
		password := clean(item.Password)
		if email == "" || password == "" {
			errs = append(errs, fmt.Sprintf("%s: email and password are required", email))
			continue
		}
		res, err := tx.Exec(`INSERT OR IGNORE INTO mailboxes (email,password,client_id,access_token,status,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, email, password, clean(item.ClientID), clean(item.AccessToken), domain.MailboxStatusNew, ts, ts)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", email, err))
			continue
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			skipped++
		} else {
			imported++
		}
	}
	return imported, skipped, errs, tx.Commit()
}

func (s *Store) ListMailboxes(status string, page, pageSize int) (domain.MailboxListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	where := ""
	args := []any{}
	if status = clean(status); status != "" {
		where = " WHERE status = ?"
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM mailboxes`+where, args...).Scan(&total); err != nil {
		return domain.MailboxListResult{}, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(`SELECT m.id,m.email,m.password,m.client_id,m.access_token,m.status,m.register_password,m.token_json,m.remark,m.last_error,m.current_step,m.current_step_index,m.current_step_total,m.proxy,m.registered_at,m.last_login_at,m.created_at,m.updated_at, COALESCE(j.id,0), COALESCE(j.type,''), COALESCE(ji.status,''), COALESCE(ji.error,'') FROM mailboxes m LEFT JOIN register_job_items ji ON ji.id = (SELECT id FROM register_job_items WHERE mailbox_id = m.id ORDER BY id DESC LIMIT 1) LEFT JOIN register_jobs j ON j.id = ji.job_id`+strings.Replace(where, "status", "m.status", 1)+` ORDER BY m.id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return domain.MailboxListResult{}, err
	}
	defer rows.Close()
	items := []domain.Mailbox{}
	for rows.Next() {
		item, err := scanMailbox(rows)
		if err != nil {
			return domain.MailboxListResult{}, err
		}
		items = append(items, item)
	}
	return domain.MailboxListResult{Total: total, Items: items}, rows.Err()
}

func (s *Store) CountMailboxesByStatus(status string) (int, error) {
	var c int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM mailboxes WHERE status = ?`, status).Scan(&c)
	return c, err
}
func (s *Store) GetMailbox(id int64) (domain.Mailbox, bool, error) {
	row := s.db.QueryRow(`SELECT m.id,m.email,m.password,m.client_id,m.access_token,m.status,m.register_password,m.token_json,m.remark,m.last_error,m.current_step,m.current_step_index,m.current_step_total,m.proxy,m.registered_at,m.last_login_at,m.created_at,m.updated_at, COALESCE(j.id,0), COALESCE(j.type,''), COALESCE(ji.status,''), COALESCE(ji.error,'') FROM mailboxes m LEFT JOIN register_job_items ji ON ji.id = (SELECT id FROM register_job_items WHERE mailbox_id = m.id ORDER BY id DESC LIMIT 1) LEFT JOIN register_jobs j ON j.id = ji.job_id WHERE m.id = ?`, id)
	item, err := scanMailbox(row)
	if err == sql.ErrNoRows {
		return domain.Mailbox{}, false, nil
	}
	return item, err == nil, err
}

func (s *Store) PickMailboxesByIDs(ids []int64) ([]domain.Mailbox, error) {
	items := make([]domain.Mailbox, 0, len(ids))
	for _, id := range ids {
		item, found, err := s.GetMailbox(id)
		if err != nil {
			return nil, err
		}
		if found {
			items = append(items, item)
		}
	}
	return items, nil
}
func (s *Store) DeleteMailbox(id int64) (bool, error) {
	res, err := s.db.Exec(`DELETE FROM mailboxes WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *Store) UpdateMailbox(id int64, updates map[string]any) (domain.Mailbox, bool, error) {
	item, found, err := s.GetMailbox(id)
	if err != nil || !found {
		return domain.Mailbox{}, found, err
	}
	if v, ok := updates["email"]; ok {
		item.Email = strings.ToLower(clean(v))
	}
	if v, ok := updates["password"]; ok {
		item.Password = clean(v)
	}
	if v, ok := updates["client_id"]; ok {
		item.ClientID = clean(v)
	}
	if v, ok := updates["access_token"]; ok {
		item.AccessToken = clean(v)
	}
	if v, ok := updates["remark"]; ok {
		item.Remark = clean(v)
	}
	if v, ok := updates["register_password"]; ok {
		item.RegisterPassword = clean(v)
	}
	if v, ok := updates["status"]; ok {
		item.Status = normalizeMailboxStatus(clean(v))
	}
	_, err = s.db.Exec(`UPDATE mailboxes SET email=?,password=?,client_id=?,access_token=?,status=?,register_password=?,remark=?,updated_at=? WHERE id=?`, item.Email, item.Password, item.ClientID, item.AccessToken, item.Status, item.RegisterPassword, item.Remark, now(), id)
	if err != nil {
		return domain.Mailbox{}, false, err
	}
	return s.GetMailbox(id)
}

func (s *Store) PickNewMailboxes(limit int) ([]domain.Mailbox, error) {
	rows, err := s.db.Query(`SELECT m.id,m.email,m.password,m.client_id,m.access_token,m.status,m.register_password,m.token_json,m.remark,m.last_error,m.current_step,m.current_step_index,m.current_step_total,m.proxy,m.registered_at,m.last_login_at,m.created_at,m.updated_at, COALESCE(j.id,0), COALESCE(j.type,''), COALESCE(ji.status,''), COALESCE(ji.error,'') FROM mailboxes m LEFT JOIN register_job_items ji ON ji.id = (SELECT id FROM register_job_items WHERE mailbox_id = m.id ORDER BY id DESC LIMIT 1) LEFT JOIN register_jobs j ON j.id = ji.job_id WHERE m.status=? ORDER BY m.id ASC LIMIT ?`, domain.MailboxStatusNew, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.Mailbox
	for rows.Next() {
		item, err := scanMailbox(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}
func (s *Store) MarkMailboxStep(id int64, status, step string, idx, total int, proxy string) error {
	_, err := s.db.Exec(`UPDATE mailboxes SET status=?, current_step=?, current_step_index=?, current_step_total=?, proxy=?, last_error='', updated_at=? WHERE id=?`, status, step, idx, total, proxy, now(), id)
	return err
}

func (s *Store) MarkMailboxLogining(id int64) error {
	_, err := s.db.Exec(`UPDATE mailboxes SET status=?, current_step='login_token_exchange', current_step_index=1, current_step_total=1, last_error='', updated_at=? WHERE id=?`, domain.MailboxStatusLogining, now(), id)
	return err
}

func (s *Store) MarkMailboxRegistered(id int64, password, tokenJSON string) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE mailboxes SET status=?, register_password=?, token_json=?, current_step='complete', current_step_index=8, current_step_total=8, last_error='', registered_at=?, updated_at=? WHERE id=?`, domain.MailboxStatusRegistered, password, tokenJSON, ts, ts, id)
	return err
}

func (s *Store) MarkMailboxLoginResult(id int64, tokenJSON string, errMessage string) error {
	ts := now()
	if errMessage != "" {
		_, err := s.db.Exec(`UPDATE mailboxes SET status=?, last_error=?, updated_at=? WHERE id=?`, domain.MailboxStatusAbnormal, errMessage, ts, id)
		return err
	}
	_, err := s.db.Exec(`UPDATE mailboxes SET status=?, token_json=?, current_step='login_complete', current_step_index=1, current_step_total=1, last_error='', last_login_at=?, updated_at=? WHERE id=?`, domain.MailboxStatusRegistered, tokenJSON, ts, ts, id)
	return err
}

func (s *Store) MarkMailboxAbnormal(id int64, message string) error {
	_, err := s.db.Exec(`UPDATE mailboxes SET status=?, last_error=?, updated_at=? WHERE id=?`, domain.MailboxStatusAbnormal, message, now(), id)
	return err
}

func (s *Store) CreateJob(count int, items []domain.Mailbox) (domain.RegisterJob, error) {
	return s.CreateTypedJob(domain.JobTypeRegister, count, items)
}

func (s *Store) CreateTypedJob(jobType string, count int, items []domain.Mailbox) (domain.RegisterJob, error) {
	if jobType == "" {
		jobType = domain.JobTypeRegister
	}
	ts := now()
	res, err := s.db.Exec(`INSERT INTO register_jobs (type,status,requested_count,total_count,started_at,created_at,updated_at) VALUES (?,?,?,?,?,?,?)`, jobType, domain.JobStatusRunning, count, len(items), ts, ts, ts)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	id, _ := res.LastInsertId()
	for _, item := range items {
		if _, err := s.db.Exec(`INSERT INTO register_job_items (job_id,mailbox_id,email,status,created_at,updated_at) VALUES (?,?,?,?,?,?)`, id, item.ID, item.Email, "pending", ts, ts); err != nil {
			return domain.RegisterJob{}, err
		}
	}
	return s.GetJob(id, false)
}
func (s *Store) RunningJobExists() (bool, error) {
	var c int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM register_jobs WHERE status=?`, domain.JobStatusRunning).Scan(&c)
	return c > 0, err
}
func (s *Store) ListJobs(page, pageSize int) (domain.JobListResult, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 5
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM register_jobs`).Scan(&total); err != nil {
		return domain.JobListResult{}, err
	}
	rows, err := s.db.Query(`SELECT id,type,status,requested_count,total_count,success_count,failed_count,success_rate,avg_duration_ms,total_duration_ms,started_at,finished_at,created_at,updated_at FROM register_jobs ORDER BY id DESC LIMIT ? OFFSET ?`, pageSize, (page-1)*pageSize)
	if err != nil {
		return domain.JobListResult{}, err
	}
	defer rows.Close()
	var items []domain.RegisterJob
	for rows.Next() {
		job, err := scanJob(rows)
		if err != nil {
			return domain.JobListResult{}, err
		}
		items = append(items, job)
	}
	return domain.JobListResult{Total: total, Items: items}, rows.Err()
}
func (s *Store) GetJob(id int64, includeItems bool) (domain.RegisterJob, error) {
	row := s.db.QueryRow(`SELECT id,type,status,requested_count,total_count,success_count,failed_count,success_rate,avg_duration_ms,total_duration_ms,started_at,finished_at,created_at,updated_at FROM register_jobs WHERE id=?`, id)
	job, err := scanJob(row)
	if err != nil {
		return domain.RegisterJob{}, err
	}
	if includeItems {
		job.Items, err = s.ListJobItems(id)
	}
	return job, err
}
func (s *Store) ListJobItems(jobID int64) ([]domain.RegisterJobItem, error) {
	rows, err := s.db.Query(`SELECT id,job_id,mailbox_id,email,status,error,duration_ms,started_at,finished_at,created_at,updated_at FROM register_job_items WHERE job_id=? ORDER BY id ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var items []domain.RegisterJobItem
	for rows.Next() {
		item, err := scanJobItem(rows)
		if err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) ListJobTokenExports(jobID int64) ([]domain.JobTokenExportItem, error) {
	rows, err := s.db.Query(`SELECT m.email, m.token_json FROM register_job_items ji JOIN mailboxes m ON m.id = ji.mailbox_id WHERE ji.job_id=? AND ji.status='success' AND TRIM(COALESCE(m.token_json, '')) <> '' ORDER BY ji.id ASC`, jobID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []domain.JobTokenExportItem{}
	for rows.Next() {
		var email, tokenJSON string
		if err := rows.Scan(&email, &tokenJSON); err != nil {
			return nil, err
		}
		var token domain.JobTokenExportItem
		if err := json.Unmarshal([]byte(tokenJSON), &token); err != nil {
			token = domain.JobTokenExportItem{"email": email, "token": tokenJSON}
		} else if _, ok := token["email"]; !ok {
			token["email"] = email
		}
		items = append(items, token)
	}
	return items, rows.Err()
}

func (s *Store) StartJobItem(jobID, mailboxID int64) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE register_job_items SET status='running', started_at=?, updated_at=? WHERE job_id=? AND mailbox_id=?`, ts, ts, jobID, mailboxID)
	return err
}
func (s *Store) UpdateJobItem(jobID, mailboxID int64, status, errMessage string, duration time.Duration) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE register_job_items SET status=?, error=?, duration_ms=?, finished_at=?, updated_at=? WHERE job_id=? AND mailbox_id=?`, status, errMessage, duration.Milliseconds(), ts, ts, jobID, mailboxID)
	return err
}
func (s *Store) RecalculateJob(jobID int64, finalStatus string) error {
	items, err := s.ListJobItems(jobID)
	if err != nil {
		return err
	}
	var current string
	var finished sql.NullString
	if err := s.db.QueryRow(`SELECT status, finished_at FROM register_jobs WHERE id=?`, jobID).Scan(&current, &finished); err != nil {
		return err
	}
	success, failed := 0, 0
	var total int64
	for _, item := range items {
		if item.Status == "success" {
			success++
		}
		if item.Status == "failed" {
			failed++
		}
		total += item.DurationMS
	}
	completed := success + failed
	avg := int64(0)
	if completed > 0 {
		avg = total / int64(completed)
	}
	rate := float64(0)
	if len(items) > 0 {
		rate = float64(success) * 100 / float64(len(items))
	}
	if finalStatus == "" {
		finalStatus = current
	}
	finish := ""
	if finalStatus != domain.JobStatusRunning {
		finish = nullString(finished)
		if finish == "" {
			finish = now()
		}
	}
	_, err = s.db.Exec(`UPDATE register_jobs SET status=?, success_count=?, failed_count=?, success_rate=?, avg_duration_ms=?, total_duration_ms=?, finished_at=?, updated_at=? WHERE id=?`, finalStatus, success, failed, rate, avg, total, finish, now(), jobID)
	return err
}
func (s *Store) StopJob(jobID int64) error {
	ts := now()
	if _, err := s.db.Exec(`UPDATE register_job_items SET status='failed', error='手动结束任务', finished_at=COALESCE(finished_at, ?), updated_at=? WHERE job_id=? AND status IN ('pending','running')`, ts, ts, jobID); err != nil {
		return err
	}
	return s.RecalculateJob(jobID, domain.JobStatusStopped)
}
func (s *Store) AddLog(log domain.RuntimeLog) (domain.RuntimeLog, error) {
	if log.Level == "" {
		log.Level = "info"
	}
	log.CreatedAt = now()
	res, err := s.db.Exec(`INSERT INTO runtime_logs (job_id,mailbox_id,email,level,step,step_index,step_total,message,created_at) VALUES (?,?,?,?,?,?,?,?,?)`, log.JobID, log.MailboxID, log.Email, log.Level, log.Step, log.StepIndex, log.StepTotal, log.Message, log.CreatedAt)
	if err != nil {
		return log, err
	}
	log.ID, _ = res.LastInsertId()
	return log, nil
}
func (s *Store) ListLogs(jobID int64, afterID int64, limit int) ([]domain.RuntimeLog, error) {
	if limit < 1 || limit > 500 {
		limit = 200
	}
	rows, err := s.db.Query(`SELECT id,job_id,mailbox_id,email,level,step,step_index,step_total,message,created_at FROM runtime_logs WHERE job_id=? AND id>? ORDER BY id ASC LIMIT ?`, jobID, afterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []domain.RuntimeLog
	for rows.Next() {
		var l domain.RuntimeLog
		if err := rows.Scan(&l.ID, &l.JobID, &l.MailboxID, &l.Email, &l.Level, &l.Step, &l.StepIndex, &l.StepTotal, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, rows.Err()
}

func scanMailbox(scanner interface{ Scan(dest ...any) error }) (domain.Mailbox, error) {
	var i domain.Mailbox
	var clientID, accessToken, registerPassword, tokenJSON, remark, lastError, step, proxy, registeredAt, lastLoginAt sql.NullString
	var lastJobType, lastJobStatus, lastJobError sql.NullString
	var lastJobID sql.NullInt64
	err := scanner.Scan(&i.ID, &i.Email, &i.Password, &clientID, &accessToken, &i.Status, &registerPassword, &tokenJSON, &remark, &lastError, &step, &i.CurrentStepIndex, &i.CurrentStepTotal, &proxy, &registeredAt, &lastLoginAt, &i.CreatedAt, &i.UpdatedAt, &lastJobID, &lastJobType, &lastJobStatus, &lastJobError)
	i.ClientID = nullString(clientID)
	i.AccessToken = nullString(accessToken)
	i.RegisterPassword = nullString(registerPassword)
	i.TokenJSON = nullString(tokenJSON)
	i.Remark = nullString(remark)
	i.LastError = nullString(lastError)
	i.CurrentStep = nullString(step)
	i.Proxy = nullString(proxy)
	i.RegisteredAt = nullString(registeredAt)
	i.LastLoginAt = nullString(lastLoginAt)
	if lastJobID.Valid {
		i.LastJobID = lastJobID.Int64
	}
	i.LastJobType = nullString(lastJobType)
	i.LastJobStatus = nullString(lastJobStatus)
	i.LastJobError = nullString(lastJobError)
	i.StatusText = domain.MailboxStatusText(i.Status)
	return i, err
}
func scanJob(scanner interface{ Scan(dest ...any) error }) (domain.RegisterJob, error) {
	var i domain.RegisterJob
	var started, finished sql.NullString
	err := scanner.Scan(&i.ID, &i.Type, &i.Status, &i.RequestedCount, &i.TotalCount, &i.SuccessCount, &i.FailedCount, &i.SuccessRate, &i.AvgDurationMS, &i.TotalDurationMS, &started, &finished, &i.CreatedAt, &i.UpdatedAt)
	i.StartedAt = nullString(started)
	i.FinishedAt = nullString(finished)
	return i, err
}
func scanJobItem(scanner interface{ Scan(dest ...any) error }) (domain.RegisterJobItem, error) {
	var i domain.RegisterJobItem
	var e, started, finished sql.NullString
	err := scanner.Scan(&i.ID, &i.JobID, &i.MailboxID, &i.Email, &i.Status, &e, &i.DurationMS, &started, &finished, &i.CreatedAt, &i.UpdatedAt)
	i.Error = nullString(e)
	i.StartedAt = nullString(started)
	i.FinishedAt = nullString(finished)
	return i, err
}
func nullString(v sql.NullString) string {
	if v.Valid {
		return v.String
	}
	return ""
}
func normalizeMailboxStatus(status string) string {
	switch status {
	case domain.MailboxStatusRegistering, domain.MailboxStatusRegistered, domain.MailboxStatusLogining, domain.MailboxStatusAbnormal:
		return status
	default:
		return domain.MailboxStatusNew
	}
}
