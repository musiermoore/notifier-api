package database

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/alexandersustavov/notifier/notifier-api/internal/domain"
)

func (db *DB) SyncTelegramDeliveryForItem(ctx context.Context, item domain.Item) error {
	var chatID *string
	var username *string
	err := db.pool.QueryRow(ctx, `
		SELECT telegram_chat_id, telegram_username
		FROM users
		WHERE id = $1
	`, item.UserID).Scan(&chatID, &username)
	if err != nil {
		return fmt.Errorf("load telegram user for delivery sync: %w", err)
	}

	if item.DeletedAt != nil || !item.DeliverToTelegram || item.RemindAt == nil || chatID == nil {
		if _, err := db.pool.Exec(ctx, `DELETE FROM telegram_deliveries WHERE item_id = $1`, item.ID); err != nil {
			return fmt.Errorf("clear telegram delivery: %w", err)
		}
		return nil
	}

	message := buildTelegramDeliveryMessage(item, username)
	_, err = db.pool.Exec(ctx, `
		INSERT INTO telegram_deliveries (
			id, item_id, user_id, chat_id, message, deliver_at, status, last_error, sent_at, created_at, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, 'pending', NULL, NULL, NOW(), NOW())
		ON CONFLICT (item_id) DO UPDATE
		SET
			chat_id = EXCLUDED.chat_id,
			message = EXCLUDED.message,
			deliver_at = EXCLUDED.deliver_at,
			status = 'pending',
			last_error = NULL,
			sent_at = NULL,
			updated_at = NOW()
	`, "tdl_"+item.ID, item.ID, item.UserID, *chatID, message, *item.RemindAt)
	if err != nil {
		return fmt.Errorf("upsert telegram delivery: %w", err)
	}

	return nil
}

func (db *DB) SyncTelegramDeliveriesForUser(ctx context.Context, userID string) error {
	rows, err := db.pool.Query(ctx, `
		SELECT id, user_id, title, body, lang, status, remind_at, repeat_rule, version, updated_at, deleted_at, source, deliver_to_telegram
		FROM items
		WHERE user_id = $1
	`, userID)
	if err != nil {
		return fmt.Errorf("load user items for delivery sync: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var item domain.Item
		if err := rows.Scan(
			&item.ID,
			&item.UserID,
			&item.Title,
			&item.Body,
			&item.Lang,
			&item.Status,
			&item.RemindAt,
			&item.RepeatRule,
			&item.Version,
			&item.UpdatedAt,
			&item.DeletedAt,
			&item.Source,
			&item.DeliverToTelegram,
		); err != nil {
			return fmt.Errorf("scan item for delivery sync: %w", err)
		}

		if err := db.SyncTelegramDeliveryForItem(ctx, item); err != nil {
			return err
		}
	}

	return rows.Err()
}

func (db *DB) ListPendingTelegramDeliveries(ctx context.Context, limit int) ([]domain.TelegramDelivery, error) {
	if limit <= 0 || limit > 100 {
		limit = 20
	}

	rows, err := db.pool.Query(ctx, `
		SELECT id, item_id, user_id, chat_id, message, deliver_at, status, last_error, sent_at, created_at, updated_at
		FROM telegram_deliveries
		WHERE status IN ('pending', 'failed') AND deliver_at <= NOW()
		ORDER BY deliver_at ASC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("list pending telegram deliveries: %w", err)
	}
	defer rows.Close()

	deliveries := make([]domain.TelegramDelivery, 0, limit)
	for rows.Next() {
		var delivery domain.TelegramDelivery
		if err := rows.Scan(
			&delivery.ID,
			&delivery.ItemID,
			&delivery.UserID,
			&delivery.ChatID,
			&delivery.Message,
			&delivery.DeliverAt,
			&delivery.Status,
			&delivery.LastError,
			&delivery.SentAt,
			&delivery.CreatedAt,
			&delivery.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("scan telegram delivery: %w", err)
		}
		deliveries = append(deliveries, delivery)
	}

	return deliveries, rows.Err()
}

func (db *DB) MarkTelegramDeliveryComplete(ctx context.Context, deliveryID string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE telegram_deliveries
		SET status = 'sent', sent_at = NOW(), last_error = NULL, updated_at = NOW()
		WHERE id = $1
	`, deliveryID)
	if err != nil {
		return fmt.Errorf("mark telegram delivery complete: %w", err)
	}
	return nil
}

func (db *DB) MarkTelegramDeliveryFailed(ctx context.Context, deliveryID, lastError string) error {
	_, err := db.pool.Exec(ctx, `
		UPDATE telegram_deliveries
		SET status = 'failed', last_error = $2, updated_at = NOW()
		WHERE id = $1
	`, deliveryID, strings.TrimSpace(lastError))
	if err != nil {
		return fmt.Errorf("mark telegram delivery failed: %w", err)
	}
	return nil
}

func buildTelegramDeliveryMessage(item domain.Item, username *string) string {
	var lines []string
	lines = append(lines, "Reminder: "+item.Title)
	if strings.TrimSpace(item.Body) != "" {
		lines = append(lines, item.Body)
	}
	if username != nil && strings.TrimSpace(*username) != "" {
		lines = append(lines, "for @"+strings.TrimSpace(*username))
	}
	lines = append(lines, "scheduled at "+item.RemindAt.UTC().Format(time.RFC3339))
	return strings.Join(lines, "\n")
}
