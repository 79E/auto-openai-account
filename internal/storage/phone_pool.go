package storage

import (
	"database/sql"
	"fmt"
	"net/url"
	"strings"

	"github.com/79E/auto-openai-account/internal/domain"
	"github.com/79E/auto-openai-account/internal/smsbiz"
)

func (s *Store) GetSMSPoolSummary(configID string) (domain.SMSPoolSummary, error) {
	configID = strings.TrimSpace(configID)
	if configID == "" {
		return domain.SMSPoolSummary{}, nil
	}
	rows, err := s.db.Query(`SELECT status, COUNT(*), COALESCE(SUM(CASE WHEN max_use_count > use_count THEN max_use_count - use_count ELSE 0 END), 0) FROM phone_pool_items WHERE sms_config_id = ? GROUP BY status`, configID)
	if err != nil {
		return domain.SMSPoolSummary{}, err
	}
	defer rows.Close()
	summary := domain.SMSPoolSummary{}
	for rows.Next() {
		var status string
		var count int
		var remaining int
		if err := rows.Scan(&status, &count, &remaining); err != nil {
			return domain.SMSPoolSummary{}, err
		}
		summary.TotalCount += count
		summary.RemainingUses += remaining
		switch status {
		case domain.PhonePoolStatusReady:
			summary.ReadyCount = count
		case domain.PhonePoolStatusReserved:
			summary.ReservedCount = count
		case domain.PhonePoolStatusUsedUp:
			summary.UsedUpCount = count
		case domain.PhonePoolStatusDisabled:
			summary.DisabledCount = count
		case domain.PhonePoolStatusError:
			summary.ErrorCount = count
		}
	}
	return summary, rows.Err()
}

func (s *Store) ListSMSPoolSummaries(configIDs []string) (map[string]domain.SMSPoolSummary, error) {
	result := make(map[string]domain.SMSPoolSummary, len(configIDs))
	for _, configID := range configIDs {
		configID = strings.TrimSpace(configID)
		if configID == "" {
			continue
		}
		summary, err := s.GetSMSPoolSummary(configID)
		if err != nil {
			return nil, err
		}
		result[configID] = summary
	}
	return result, nil
}

func (s *Store) SyncPhonePoolMaxUseCount(configID string, maxUseCount int) error {
	configID = strings.TrimSpace(configID)
	if configID == "" || maxUseCount < 1 {
		return nil
	}
	_, err := s.db.Exec(`UPDATE phone_pool_items
		SET
			max_use_count = ?,
			status = CASE
				WHEN status IN (?, ?) THEN status
				WHEN use_count >= ? THEN ?
				ELSE ?
			END,
			updated_at = ?
		WHERE sms_config_id = ?`,
		maxUseCount,
		domain.PhonePoolStatusReserved,
		domain.PhonePoolStatusDisabled,
		maxUseCount,
		domain.PhonePoolStatusUsedUp,
		domain.PhonePoolStatusReady,
		now(),
		configID,
	)
	return err
}

func (s *Store) ImportPhonePoolItems(configID string, maxUseCount int, text string) (int, int, []string, error) {
	configID = strings.TrimSpace(configID)
	if configID == "" {
		return 0, 0, nil, fmt.Errorf("sms config id is required")
	}
	if maxUseCount < 1 {
		maxUseCount = 1
	}
	items, errs := parsePhonePoolImport(text)
	tx, err := s.db.Begin()
	if err != nil {
		return 0, 0, errs, err
	}
	defer tx.Rollback()
	imported, skipped := 0, 0
	ts := now()
	for _, item := range items {
		res, err := tx.Exec(`INSERT OR IGNORE INTO phone_pool_items (sms_config_id, phone_number, code_fetch_url, status, use_count, max_use_count, created_at, updated_at) VALUES (?, ?, ?, ?, 0, ?, ?, ?)`, configID, item.PhoneNumber, item.CodeFetchURL, domain.PhonePoolStatusReady, maxUseCount, ts, ts)
		if err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", item.PhoneNumber, err))
			continue
		}
		rows, _ := res.RowsAffected()
		if rows == 0 {
			skipped++
			continue
		}
		imported++
	}
	return imported, skipped, errs, tx.Commit()
}

func (s *Store) ListPhonePoolItems(configID string, status string, page, pageSize int) (domain.PhonePoolListResult, error) {
	configID = strings.TrimSpace(configID)
	if configID == "" {
		return domain.PhonePoolListResult{}, fmt.Errorf("sms config id is required")
	}
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 200 {
		pageSize = 50
	}
	where := " WHERE sms_config_id = ?"
	args := []any{configID}
	status = normalizePhonePoolStatus(status)
	if status != "" {
		where += " AND status = ?"
		args = append(args, status)
	}
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM phone_pool_items`+where, args...).Scan(&total); err != nil {
		return domain.PhonePoolListResult{}, err
	}
	args = append(args, pageSize, (page-1)*pageSize)
	rows, err := s.db.Query(`SELECT id, sms_config_id, phone_number, code_fetch_url, status, use_count, max_use_count, last_error, last_job_id, last_mailbox_id, reserved_at, last_used_at, created_at, updated_at FROM phone_pool_items`+where+` ORDER BY id DESC LIMIT ? OFFSET ?`, args...)
	if err != nil {
		return domain.PhonePoolListResult{}, err
	}
	defer rows.Close()
	items := []domain.PhonePoolItem{}
	for rows.Next() {
		item, err := scanPhonePoolItem(rows)
		if err != nil {
			return domain.PhonePoolListResult{}, err
		}
		items = append(items, item)
	}
	return domain.PhonePoolListResult{Total: total, Items: items}, rows.Err()
}

func (s *Store) ReserveNextPhonePoolItem(configID string) (domain.PhonePoolItem, error) {
	configID = strings.TrimSpace(configID)
	if configID == "" {
		return domain.PhonePoolItem{}, fmt.Errorf("sms config id is required")
	}
	tx, err := s.db.Begin()
	if err != nil {
		return domain.PhonePoolItem{}, err
	}
	defer tx.Rollback()
	row := tx.QueryRow(`SELECT id, sms_config_id, phone_number, code_fetch_url, status, use_count, max_use_count, last_error, last_job_id, last_mailbox_id, reserved_at, last_used_at, created_at, updated_at FROM phone_pool_items WHERE sms_config_id = ? AND status = ? AND use_count < max_use_count ORDER BY use_count ASC, id ASC LIMIT 1`, configID, domain.PhonePoolStatusReady)
	item, err := scanPhonePoolItem(row)
	if err == sql.ErrNoRows {
		return domain.PhonePoolItem{}, fmt.Errorf("no usable phone numbers available")
	}
	if err != nil {
		return domain.PhonePoolItem{}, err
	}
	ts := now()
	if _, err := tx.Exec(`UPDATE phone_pool_items SET status = ?, reserved_at = ?, updated_at = ? WHERE id = ?`, domain.PhonePoolStatusReserved, ts, ts, item.ID); err != nil {
		return domain.PhonePoolItem{}, err
	}
	item.Status = domain.PhonePoolStatusReserved
	item.ReservedAt = ts
	item.UpdatedAt = ts
	if err := tx.Commit(); err != nil {
		return domain.PhonePoolItem{}, err
	}
	return item, nil
}

func (s *Store) GetPhonePoolItem(id int64) (domain.PhonePoolItem, error) {
	row := s.db.QueryRow(`SELECT id, sms_config_id, phone_number, code_fetch_url, status, use_count, max_use_count, last_error, last_job_id, last_mailbox_id, reserved_at, last_used_at, created_at, updated_at FROM phone_pool_items WHERE id = ?`, id)
	return scanPhonePoolItem(row)
}

func (s *Store) DeletePhonePoolItem(id int64) (bool, error) {
	item, err := s.GetPhonePoolItem(id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	if item.Status == domain.PhonePoolStatusReserved {
		return false, fmt.Errorf("使用中的手机号不能删除")
	}
	res, err := s.db.Exec(`DELETE FROM phone_pool_items WHERE id = ?`, id)
	if err != nil {
		return false, err
	}
	rows, _ := res.RowsAffected()
	return rows > 0, nil
}

func (s *Store) MarkPhonePoolItemSubmitted(itemID int64, jobID int64, mailboxID int64) error {
	_, err := s.db.Exec(`UPDATE phone_pool_items SET use_count = use_count + 1, last_job_id = ?, last_mailbox_id = ?, updated_at = ? WHERE id = ?`, jobID, mailboxID, now(), itemID)
	return err
}

func (s *Store) CompletePhonePoolItem(itemID int64) error {
	item, err := s.GetPhonePoolItem(itemID)
	if err != nil {
		return err
	}
	status := domain.PhonePoolStatusReady
	if item.UseCount >= item.MaxUseCount {
		status = domain.PhonePoolStatusUsedUp
	}
	ts := now()
	_, err = s.db.Exec(`UPDATE phone_pool_items SET status = ?, last_error = '', reserved_at = NULL, last_used_at = ?, updated_at = ? WHERE id = ?`, status, ts, ts, itemID)
	return err
}

func (s *Store) ExhaustPhonePoolItem(itemID int64, errMessage string) error {
	ts := now()
	_, err := s.db.Exec(`UPDATE phone_pool_items SET status = ?, use_count = max_use_count, last_error = ?, reserved_at = NULL, last_used_at = ?, updated_at = ? WHERE id = ?`, domain.PhonePoolStatusUsedUp, strings.TrimSpace(errMessage), ts, ts, itemID)
	return err
}

func (s *Store) ReleasePhonePoolItem(itemID int64, errMessage string) error {
	_, err := s.db.Exec(`UPDATE phone_pool_items SET status = ?, last_error = ?, reserved_at = NULL, updated_at = ? WHERE id = ?`, domain.PhonePoolStatusReady, strings.TrimSpace(errMessage), now(), itemID)
	return err
}

func (s *Store) FailPhonePoolItem(itemID int64, disable bool, errMessage string) error {
	item, err := s.GetPhonePoolItem(itemID)
	if err != nil {
		return err
	}
	status := domain.PhonePoolStatusReady
	if disable {
		status = domain.PhonePoolStatusDisabled
	} else if item.UseCount >= item.MaxUseCount {
		status = domain.PhonePoolStatusUsedUp
	}
	_, err = s.db.Exec(`UPDATE phone_pool_items SET status = ?, last_error = ?, reserved_at = NULL, updated_at = ? WHERE id = ?`, status, strings.TrimSpace(errMessage), now(), itemID)
	return err
}

func (s *Store) CreatePhonePoolAttempt(itemID int64, configID string, jobID int64, mailboxID int64, phoneNumber string) (int64, error) {
	res, err := s.db.Exec(`INSERT INTO phone_pool_attempts (phone_pool_item_id, sms_config_id, job_id, mailbox_id, phone_number, result, created_at) VALUES (?, ?, ?, ?, ?, ?, ?)`, itemID, strings.TrimSpace(configID), jobID, mailboxID, strings.TrimSpace(phoneNumber), "running", now())
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

func (s *Store) FinishPhonePoolAttempt(attemptID int64, result string, errorCode string, errorMessage string, verificationCode string) error {
	_, err := s.db.Exec(`UPDATE phone_pool_attempts SET result = ?, error_code = ?, error_message = ?, verification_code = ?, finished_at = ? WHERE id = ?`, strings.TrimSpace(result), strings.TrimSpace(errorCode), strings.TrimSpace(errorMessage), strings.TrimSpace(verificationCode), now(), attemptID)
	return err
}

func scanPhonePoolItem(scanner interface{ Scan(dest ...any) error }) (domain.PhonePoolItem, error) {
	var item domain.PhonePoolItem
	var lastError, reservedAt, lastUsedAt sql.NullString
	var lastJobID, lastMailboxID sql.NullInt64
	err := scanner.Scan(&item.ID, &item.SMSConfigID, &item.PhoneNumber, &item.CodeFetchURL, &item.Status, &item.UseCount, &item.MaxUseCount, &lastError, &lastJobID, &lastMailboxID, &reservedAt, &lastUsedAt, &item.CreatedAt, &item.UpdatedAt)
	item.LastError = nullString(lastError)
	item.ReservedAt = nullString(reservedAt)
	item.LastUsedAt = nullString(lastUsedAt)
	if lastJobID.Valid {
		item.LastJobID = lastJobID.Int64
	}
	if lastMailboxID.Valid {
		item.LastMailboxID = lastMailboxID.Int64
	}
	return item, err
}

func parsePhonePoolImport(text string) ([]domain.PhonePoolItem, []string) {
	items := []domain.PhonePoolItem{}
	errList := []string{}
	for idx, line := range strings.Split(text, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		phone, link, err := splitPhonePoolLine(line)
		if err != nil {
			errList = append(errList, fmt.Sprintf("line %d: %v", idx+1, err))
			continue
		}
		phone, err = normalizePhonePoolNumber(phone)
		if err != nil {
			errList = append(errList, fmt.Sprintf("line %d: %v", idx+1, err))
			continue
		}
		link, err = normalizePhonePoolURL(link)
		if err != nil {
			errList = append(errList, fmt.Sprintf("line %d: %v", idx+1, err))
			continue
		}
		items = append(items, domain.PhonePoolItem{PhoneNumber: phone, CodeFetchURL: link})
	}
	return items, errList
}

func splitPhonePoolLine(line string) (string, string, error) {
	if left, right, ok := splitPhonePoolLineByDelimiter(line, "----"); ok {
		return left, right, nil
	}
	if left, right, ok := splitPhonePoolLineByDelimiter(line, "|"); ok {
		return left, right, nil
	}
	if left, right, ok := splitPhonePoolLineByColon(line); ok {
		return left, right, nil
	}
	return "", "", fmt.Errorf("invalid format")
}

func splitPhonePoolLineByDelimiter(line string, delimiter string) (string, string, bool) {
	parts := strings.SplitN(line, delimiter, 2)
	if len(parts) != 2 {
		return "", "", false
	}
	left := strings.TrimSpace(parts[0])
	right := strings.TrimSpace(parts[1])
	if left == "" || right == "" {
		return "", "", false
	}
	return left, right, true
}

func splitPhonePoolLineByColon(line string) (string, string, bool) {
	for _, prefix := range []string{"https://", "http://"} {
		idx := strings.Index(line, prefix)
		if idx <= 0 {
			continue
		}
		left := strings.TrimSpace(strings.TrimSuffix(line[:idx], ":"))
		right := strings.TrimSpace(line[idx:])
		if left == "" || right == "" {
			return "", "", false
		}
		return left, right, true
	}
	return "", "", false
}

func normalizePhonePoolNumber(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("phone number is required")
	}
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	value := digits.String()
	if value == "" {
		return "", fmt.Errorf("invalid phone number")
	}
	return "+" + value, nil
}

func normalizePhonePoolURL(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", fmt.Errorf("code fetch url is required")
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("invalid code fetch url")
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", fmt.Errorf("code fetch url must use http or https")
	}
	if strings.TrimSpace(parsed.Host) == "" {
		return "", fmt.Errorf("code fetch url host is required")
	}
	return parsed.String(), nil
}

func normalizePhonePoolStatus(status string) string {
	status = strings.TrimSpace(status)
	switch status {
	case domain.PhonePoolStatusReady, domain.PhonePoolStatusReserved, domain.PhonePoolStatusUsedUp, domain.PhonePoolStatusDisabled, domain.PhonePoolStatusError:
		return status
	default:
		return ""
	}
}

func ExtractPhonePoolCode(body string) string {
	return smsbiz.ExtractCodeFromText(body)
}
