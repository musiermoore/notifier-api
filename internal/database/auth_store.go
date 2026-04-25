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
	"golang.org/x/crypto/bcrypt"
)

var ErrInvalidCredentials = errors.New("invalid credentials")
var ErrNotFound = errors.New("not found")

func (db *DB) CreateUser(ctx context.Context, email, password, name, lang string, ttl time.Duration) (domain.User, string, error) {
	email = normalizeEmail(email)
	if lang == "" {
		lang = "ru"
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("hash password: %w", err)
	}

	user := domain.User{
		ID:          newID("usr"),
		Email:       email,
		Name:        strings.TrimSpace(name),
		Lang:        lang,
		CreatedAt:   time.Now().UTC(),
		LastLoginAt: time.Now().UTC(),
	}

	_, err = db.pool.Exec(ctx, `
		INSERT INTO users (id, email, password_hash, name, lang, created_at, updated_at, last_login_at)
		VALUES ($1, $2, $3, $4, $5, NOW(), NOW(), NOW())
	`, user.ID, user.Email, string(hash), user.Name, user.Lang)
	if err != nil {
		return domain.User{}, "", fmt.Errorf("insert user: %w", err)
	}

	token, err := db.createSession(ctx, user.ID, ttl)
	if err != nil {
		return domain.User{}, "", err
	}

	return user, token, nil
}

func (db *DB) Login(ctx context.Context, email, password string, ttl time.Duration) (domain.User, string, error) {
	email = normalizeEmail(email)

	var (
		user             domain.User
		passwordHash     string
		telegramChat     *string
		telegramUsername *string
	)

	err := db.pool.QueryRow(ctx, `
		SELECT id, email, name, lang, created_at, last_login_at, telegram_chat_id, telegram_username, password_hash
		FROM users
		WHERE email = $1
	`, email).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.Lang,
		&user.CreatedAt,
		&user.LastLoginAt,
		&telegramChat,
		&telegramUsername,
		&passwordHash,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, "", ErrInvalidCredentials
		}

		return domain.User{}, "", fmt.Errorf("find user: %w", err)
	}

	if bcrypt.CompareHashAndPassword([]byte(passwordHash), []byte(password)) != nil {
		return domain.User{}, "", ErrInvalidCredentials
	}

	user.TelegramChat = telegramChat
	user.TelegramUsername = telegramUsername
	if _, err := db.pool.Exec(ctx, `UPDATE users SET last_login_at = NOW(), updated_at = NOW() WHERE id = $1`, user.ID); err != nil {
		return domain.User{}, "", fmt.Errorf("update last login: %w", err)
	}

	token, err := db.createSession(ctx, user.ID, ttl)
	if err != nil {
		return domain.User{}, "", err
	}

	return user, token, nil
}

func (db *DB) FindUserByToken(ctx context.Context, token string) (domain.User, error) {
	var user domain.User
	var telegramChat *string
	var telegramUsername *string

	err := db.pool.QueryRow(ctx, `
		SELECT u.id, u.email, u.name, u.lang, u.created_at, u.last_login_at, u.telegram_chat_id, u.telegram_username
		FROM sessions s
		JOIN users u ON u.id = s.user_id
		WHERE s.token = $1 AND s.expires_at > NOW()
	`, token).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.Lang,
		&user.CreatedAt,
		&user.LastLoginAt,
		&telegramChat,
		&telegramUsername,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}

		return domain.User{}, fmt.Errorf("find session user: %w", err)
	}

	user.TelegramChat = telegramChat
	user.TelegramUsername = telegramUsername
	return user, nil
}

func (db *DB) FindUserByTelegramChat(ctx context.Context, chatID string) (domain.User, error) {
	var user domain.User
	var telegramChat *string
	var telegramUsername *string

	err := db.pool.QueryRow(ctx, `
		SELECT id, email, name, lang, created_at, last_login_at, telegram_chat_id, telegram_username
		FROM users
		WHERE telegram_chat_id = $1
	`, strings.TrimSpace(chatID)).Scan(
		&user.ID,
		&user.Email,
		&user.Name,
		&user.Lang,
		&user.CreatedAt,
		&user.LastLoginAt,
		&telegramChat,
		&telegramUsername,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return domain.User{}, ErrNotFound
		}

		return domain.User{}, fmt.Errorf("find telegram user: %w", err)
	}

	user.TelegramChat = telegramChat
	user.TelegramUsername = telegramUsername
	return user, nil
}

func (db *DB) DeleteSession(ctx context.Context, token string) error {
	_, err := db.pool.Exec(ctx, `DELETE FROM sessions WHERE token = $1`, token)
	if err != nil {
		return fmt.Errorf("delete session: %w", err)
	}

	return nil
}

func (db *DB) createSession(ctx context.Context, userID string, ttl time.Duration) (string, error) {
	token := newID("tok")
	expiresAt := time.Now().UTC().Add(ttl)
	_, err := db.pool.Exec(ctx, `
		INSERT INTO sessions (token, user_id, expires_at)
		VALUES ($1, $2, $3)
	`, token, userID, expiresAt)
	if err != nil {
		return "", fmt.Errorf("create session: %w", err)
	}

	return token, nil
}

func newID(prefix string) string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		panic(err)
	}

	return prefix + "_" + hex.EncodeToString(buf)
}

func normalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}
