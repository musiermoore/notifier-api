package database

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/alexandersustavov/notifier/notifier-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type TelegramLinkStatus struct {
	Connected        bool                     `json:"connected"`
	BotUsername      string                   `json:"bot_username"`
	TelegramChat     *string                  `json:"telegram_chat,omitempty"`
	TelegramUsername *string                  `json:"telegram_username,omitempty"`
	PendingCode      *domain.TelegramLinkCode `json:"pending_code,omitempty"`
}

func (db *DB) GetTelegramLinkStatus(ctx context.Context, userID, botUsername string) (TelegramLinkStatus, error) {
	var status TelegramLinkStatus
	status.BotUsername = botUsername

	var chatID *string
	var username *string
	err := db.pool.QueryRow(ctx, `
		SELECT telegram_chat_id, telegram_username
		FROM users
		WHERE id = $1
	`, userID).Scan(&chatID, &username)
	if err != nil {
		return TelegramLinkStatus{}, fmt.Errorf("load telegram status: %w", err)
	}

	status.TelegramChat = chatID
	status.TelegramUsername = username
	status.Connected = chatID != nil

	var pending domain.TelegramLinkCode
	err = db.pool.QueryRow(ctx, `
		SELECT code, user_id, expires_at, used_at
		FROM telegram_link_codes
		WHERE user_id = $1 AND used_at IS NULL AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`, userID).Scan(&pending.Code, &pending.UserID, &pending.ExpiresAt, &pending.UsedAt)
	if err == nil {
		status.PendingCode = &pending
		return status, nil
	}

	if errors.Is(err, pgx.ErrNoRows) {
		return status, nil
	}

	return TelegramLinkStatus{}, fmt.Errorf("load pending link code: %w", err)
}

func (db *DB) CreateTelegramLinkCode(ctx context.Context, userID string, ttl time.Duration) (domain.TelegramLinkCode, error) {
	if _, err := db.pool.Exec(ctx, `
		DELETE FROM telegram_link_codes
		WHERE user_id = $1 OR expires_at <= NOW() OR used_at IS NOT NULL
	`, userID); err != nil {
		return domain.TelegramLinkCode{}, fmt.Errorf("clear old link codes: %w", err)
	}

	linkCode := domain.TelegramLinkCode{
		Code:      newTelegramCode(),
		UserID:    userID,
		ExpiresAt: time.Now().UTC().Add(ttl),
	}

	_, err := db.pool.Exec(ctx, `
		INSERT INTO telegram_link_codes (code, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, linkCode.Code, linkCode.UserID, linkCode.ExpiresAt)
	if err != nil {
		return domain.TelegramLinkCode{}, fmt.Errorf("create link code: %w", err)
	}

	return linkCode, nil
}

func (db *DB) ConsumeTelegramLinkCode(ctx context.Context, code, chatID string, username *string) (domain.User, error) {
	tx, err := db.pool.Begin(ctx)
	if err != nil {
		return domain.User{}, fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	var userID string
	err = tx.QueryRow(ctx, `
		SELECT user_id
		FROM telegram_link_codes
		WHERE code = $1 AND used_at IS NULL AND expires_at > NOW()
		FOR UPDATE
	`, strings.TrimSpace(code)).Scan(&userID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}
		return domain.User{}, fmt.Errorf("find link code: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE users
		SET telegram_chat_id = $2, telegram_username = $3, updated_at = NOW()
		WHERE id = $1
	`, userID, chatID, username); err != nil {
		return domain.User{}, fmt.Errorf("link telegram user: %w", err)
	}

	if _, err := tx.Exec(ctx, `
		UPDATE telegram_link_codes
		SET used_at = NOW()
		WHERE code = $1
	`, strings.TrimSpace(code)); err != nil {
		return domain.User{}, fmt.Errorf("mark code as used: %w", err)
	}

	var user domain.User
	var linkedChatID *string
	var linkedUsername *string
	if err := tx.QueryRow(ctx, `
		SELECT id, email, name, lang, created_at, last_login_at, telegram_chat_id, telegram_username
		FROM users
		WHERE id = $1
	`, userID).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.Lang,
		&user.CreatedAt,
		&user.LastLoginAt,
		&linkedChatID,
		&linkedUsername,
	); err != nil {
		return domain.User{}, fmt.Errorf("load linked user: %w", err)
	}
	user.TelegramChat = linkedChatID
	user.TelegramUsername = linkedUsername

	if err := tx.Commit(ctx); err != nil {
		return domain.User{}, fmt.Errorf("commit telegram link: %w", err)
	}

	if err := db.SyncTelegramDeliveriesForUser(ctx, user.ID); err != nil {
		return domain.User{}, err
	}

	return user, nil
}

func (db *DB) UnlinkTelegram(ctx context.Context, userID string) error {
	if _, err := db.pool.Exec(ctx, `
		UPDATE users
		SET telegram_chat_id = NULL, telegram_username = NULL, updated_at = NOW()
		WHERE id = $1
	`, userID); err != nil {
		return fmt.Errorf("unlink telegram: %w", err)
	}

	if _, err := db.pool.Exec(ctx, `
		DELETE FROM telegram_link_codes
		WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("clear telegram codes: %w", err)
	}

	if _, err := db.pool.Exec(ctx, `
		DELETE FROM telegram_deliveries
		WHERE user_id = $1
	`, userID); err != nil {
		return fmt.Errorf("clear telegram deliveries: %w", err)
	}

	return nil
}

func newTelegramCode() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}

	return strings.ToUpper(hex.EncodeToString(buf))
}
