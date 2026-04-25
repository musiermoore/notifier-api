package database

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/alexandersustavov/notifier/notifier-api/internal/domain"
	"github.com/jackc/pgx/v5"
)

type CreateItemParams struct {
	UserID            string
	Title             string
	Body              string
	Lang              string
	Status            string
	RemindAt          *time.Time
	RepeatRule        *string
	Source            string
	DeliverToTelegram bool
}

type UpdateItemParams struct {
	UserID            string
	ID                string
	Title             string
	Body              string
	Lang              string
	Status            string
	RemindAt          *time.Time
	RepeatRule        *string
	Source            string
	DeliverToTelegram bool
}

func (db *DB) CreateItem(ctx context.Context, params CreateItemParams) (domain.Item, error) {
	item := domain.Item{
		ID:                newID("itm"),
		UserID:            params.UserID,
		Title:             params.Title,
		Body:              params.Body,
		Lang:              params.Lang,
		Status:            params.Status,
		RemindAt:          params.RemindAt,
		RepeatRule:        params.RepeatRule,
		Source:            params.Source,
		DeliverToTelegram: params.DeliverToTelegram,
	}

	err := db.pool.QueryRow(ctx, `
		INSERT INTO items (
			id, user_id, title, body, lang, status, remind_at, repeat_rule, deliver_to_telegram, source,
			version, updated_at
		)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 1, NOW())
		RETURNING version, updated_at, deleted_at
	`,
		item.ID,
		item.UserID,
		item.Title,
		item.Body,
		item.Lang,
		item.Status,
		item.RemindAt,
		item.RepeatRule,
		item.DeliverToTelegram,
		item.Source,
	).Scan(&item.Version, &item.UpdatedAt, &item.DeletedAt)
	if err != nil {
		return domain.Item{}, fmt.Errorf("create item: %w", err)
	}

	if err := db.SyncTelegramDeliveryForItem(ctx, item); err != nil {
		return domain.Item{}, err
	}

	return item, nil
}

func (db *DB) ListItems(ctx context.Context, userID string, includeDeleted bool, since *time.Time) ([]domain.Item, error) {
	query := `
		SELECT id, user_id, title, body, lang, status, remind_at, repeat_rule, version, updated_at, deleted_at, source, deliver_to_telegram
		FROM items
		WHERE user_id = $1
	`
	args := []any{userID}

	if !includeDeleted {
		query += ` AND deleted_at IS NULL`
	}

	if since != nil {
		query += fmt.Sprintf(" AND updated_at >= $%d", len(args)+1)
		args = append(args, *since)
	}

	query += ` ORDER BY updated_at DESC`

	rows, err := db.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("list items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Item, 0)
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
			return nil, fmt.Errorf("scan item: %w", err)
		}

		items = append(items, item)
	}

	if rows.Err() != nil {
		return nil, fmt.Errorf("iterate items: %w", rows.Err())
	}

	return items, nil
}

func (db *DB) ListRecentItems(ctx context.Context, userID string, limit int) ([]domain.Item, error) {
	if limit <= 0 || limit > 50 {
		limit = 10
	}

	rows, err := db.pool.Query(ctx, `
		SELECT id, user_id, title, body, lang, status, remind_at, repeat_rule, version, updated_at, deleted_at, source, deliver_to_telegram
		FROM items
		WHERE user_id = $1 AND deleted_at IS NULL
		ORDER BY updated_at DESC
		LIMIT $2
	`, userID, limit)
	if err != nil {
		return nil, fmt.Errorf("list recent items: %w", err)
	}
	defer rows.Close()

	items := make([]domain.Item, 0, limit)
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
			return nil, fmt.Errorf("scan recent item: %w", err)
		}

		items = append(items, item)
	}

	return items, rows.Err()
}

func (db *DB) GetItem(ctx context.Context, userID, itemID string) (domain.Item, error) {
	var item domain.Item
	err := db.pool.QueryRow(ctx, `
		SELECT id, user_id, title, body, lang, status, remind_at, repeat_rule, version, updated_at, deleted_at, source, deliver_to_telegram
		FROM items
		WHERE user_id = $1 AND id = $2
	`, userID, itemID).Scan(
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Item{}, ErrNotFound
		}

		return domain.Item{}, fmt.Errorf("get item: %w", err)
	}

	return item, nil
}

func (db *DB) UpdateItem(ctx context.Context, params UpdateItemParams) (domain.Item, error) {
	var item domain.Item
	err := db.pool.QueryRow(ctx, `
		UPDATE items
		SET
			title = $3,
			body = $4,
			lang = $5,
			status = $6,
			remind_at = $7,
			repeat_rule = $8,
			source = $9,
			deliver_to_telegram = $10,
			version = version + 1,
			updated_at = NOW()
		WHERE user_id = $1 AND id = $2
		RETURNING id, user_id, title, body, lang, status, remind_at, repeat_rule, version, updated_at, deleted_at, source, deliver_to_telegram
	`,
		params.UserID,
		params.ID,
		params.Title,
		params.Body,
		params.Lang,
		params.Status,
		params.RemindAt,
		params.RepeatRule,
		params.Source,
		params.DeliverToTelegram,
	).Scan(
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.Item{}, ErrNotFound
		}

		return domain.Item{}, fmt.Errorf("update item: %w", err)
	}

	if err := db.SyncTelegramDeliveryForItem(ctx, item); err != nil {
		return domain.Item{}, err
	}

	return item, nil
}

func (db *DB) DeleteItem(ctx context.Context, userID, itemID string) error {
	var item domain.Item
	err := db.pool.QueryRow(ctx, `
		UPDATE items
		SET deleted_at = NOW(), updated_at = NOW(), version = version + 1
		WHERE user_id = $1 AND id = $2 AND deleted_at IS NULL
		RETURNING id, user_id, title, body, lang, status, remind_at, repeat_rule, version, updated_at, deleted_at, source, deliver_to_telegram
	`, userID, itemID).Scan(
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
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return fmt.Errorf("delete item: %w", err)
	}

	if err := db.SyncTelegramDeliveryForItem(ctx, item); err != nil {
		return err
	}

	return nil
}
