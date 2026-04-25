package http

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/alexandersustavov/notifier/notifier-api/internal/config"
	"github.com/alexandersustavov/notifier/notifier-api/internal/database"
	"github.com/alexandersustavov/notifier/notifier-api/internal/domain"
)

type App struct {
	cfg   config.Config
	store *database.DB
}

type contextKey string

const userContextKey contextKey = "user"

func NewRouter(cfg config.Config, store *database.DB) http.Handler {
	app := &App{
		cfg:   cfg,
		store: store,
	}

	mux := http.NewServeMux()

	mux.HandleFunc("GET /health", app.handleHealth)
	mux.HandleFunc("GET /v1/meta", app.handleMeta)
	mux.HandleFunc("POST /v1/auth/register", app.handleRegister)
	mux.HandleFunc("POST /v1/auth/login", app.handleLogin)
	mux.Handle("GET /v1/auth/me", app.requireAuth(http.HandlerFunc(app.handleMe)))
	mux.Handle("POST /v1/auth/logout", app.requireAuth(http.HandlerFunc(app.handleLogout)))
	mux.Handle("GET /v1/telegram/link", app.requireAuth(http.HandlerFunc(app.handleTelegramLinkStatus)))
	mux.Handle("POST /v1/telegram/link-code", app.requireAuth(http.HandlerFunc(app.handleTelegramLinkCode)))
	mux.Handle("DELETE /v1/telegram/link", app.requireAuth(http.HandlerFunc(app.handleTelegramUnlink)))
	mux.HandleFunc("POST /v1/telegram/consume-link", app.handleTelegramConsumeLink)
	mux.Handle("GET /v1/internal/telegram/deliveries/pending", app.requireBotService(http.HandlerFunc(app.handlePendingTelegramDeliveries)))
	mux.Handle("POST /v1/internal/telegram/deliveries/", app.requireBotService(http.HandlerFunc(app.handleTelegramDeliveryAction)))
	mux.Handle("GET /v1/items", app.requireAuth(http.HandlerFunc(app.handleListItems)))
	mux.Handle("POST /v1/items", app.requireAuth(http.HandlerFunc(app.handleCreateItem)))
	mux.Handle("GET /v1/sync/pull", app.requireAuth(http.HandlerFunc(app.handleSyncPull)))
	mux.Handle("GET /v1/items/", app.requireAuth(http.HandlerFunc(app.handleItemByID)))
	mux.Handle("PATCH /v1/items/", app.requireAuth(http.HandlerFunc(app.handleItemByID)))
	mux.Handle("DELETE /v1/items/", app.requireAuth(http.HandlerFunc(app.handleItemByID)))

	return withCORS(mux)
}

func (app *App) handleHealth(w http.ResponseWriter, _ *http.Request) {
	status := http.StatusOK
	if err := app.store.Pool().Ping(context.Background()); err != nil {
		status = http.StatusServiceUnavailable
	}

	writeJSON(w, status, map[string]any{
		"service": "notifier-api",
		"status":  map[bool]string{true: "ok", false: "degraded"}[status == http.StatusOK],
		"env":     app.cfg.Env,
		"lang":    app.cfg.DefaultLang,
		"time":    time.Now().UTC(),
	})
}

func (app *App) handleMeta(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"default_language": app.cfg.DefaultLang,
		"supported_languages": []string{
			"ru",
			"en",
		},
		"entity": domain.ItemSchema(),
	})
}

func (app *App) handleRegister(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
		Name     string `json:"name"`
		Lang     string `json:"lang"`
	}

	if !decodeJSON(w, r, &input) {
		return
	}

	if strings.TrimSpace(input.Email) == "" || strings.TrimSpace(input.Password) == "" || strings.TrimSpace(input.Name) == "" {
		writeError(w, http.StatusBadRequest, "email, password, and name are required")
		return
	}

	user, token, err := app.store.CreateUser(
		r.Context(),
		input.Email,
		input.Password,
		input.Name,
		fallbackLang(input.Lang, app.cfg.DefaultLang),
		app.cfg.SessionTTL,
	)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"user":         user,
		"access_token": token,
		"token_type":   "Bearer",
		"default_lang": app.cfg.DefaultLang,
	})
}

func (app *App) handleLogin(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if !decodeJSON(w, r, &input) {
		return
	}

	user, token, err := app.store.Login(r.Context(), input.Email, input.Password, app.cfg.SessionTTL)
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, database.ErrInvalidCredentials) {
			status = http.StatusUnauthorized
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"user":         user,
		"access_token": token,
		"token_type":   "Bearer",
	})
}

func (app *App) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"user": currentUser(r.Context()),
	})
}

func (app *App) handleLogout(w http.ResponseWriter, r *http.Request) {
	token := bearerToken(r.Header.Get("Authorization"))
	if err := app.store.DeleteSession(r.Context(), token); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleTelegramLinkStatus(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	status, err := app.store.GetTelegramLinkStatus(r.Context(), user.ID, app.cfg.TelegramBotUsername)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, status)
}

func (app *App) handleTelegramLinkCode(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	linkCode, err := app.store.CreateTelegramLinkCode(r.Context(), user.ID, 10*time.Minute)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{
		"code":         linkCode.Code,
		"expires_at":   linkCode.ExpiresAt,
		"bot_username": app.cfg.TelegramBotUsername,
	})
}

func (app *App) handleTelegramUnlink(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	if err := app.store.UnlinkTelegram(r.Context(), user.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleTelegramConsumeLink(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Code     string  `json:"code"`
		ChatID   string  `json:"chat_id"`
		Username *string `json:"username"`
	}

	if !decodeJSON(w, r, &input) {
		return
	}

	if strings.TrimSpace(input.Code) == "" || strings.TrimSpace(input.ChatID) == "" {
		writeError(w, http.StatusBadRequest, "code and chat_id are required")
		return
	}

	user, err := app.store.ConsumeTelegramLinkCode(r.Context(), input.Code, input.ChatID, normalizeOptionalString(input.Username))
	if err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, database.ErrNotFound) {
			status = http.StatusNotFound
		}
		writeError(w, status, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"ok":   true,
		"user": user,
	})
}

func (app *App) handlePendingTelegramDeliveries(w http.ResponseWriter, r *http.Request) {
	limit := 20
	deliveries, err := app.store.ListPendingTelegramDeliveries(r.Context(), limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"deliveries": deliveries,
	})
}

func (app *App) handleTelegramDeliveryAction(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/v1/internal/telegram/deliveries/")
	parts := strings.Split(strings.Trim(path, "/"), "/")
	if len(parts) != 2 {
		writeError(w, http.StatusNotFound, "delivery route not found")
		return
	}

	deliveryID := parts[0]
	action := parts[1]

	switch action {
	case "complete":
		if err := app.store.MarkTelegramDeliveryComplete(r.Context(), deliveryID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	case "fail":
		var input struct {
			Error string `json:"error"`
		}
		if !decodeJSON(w, r, &input) {
			return
		}
		if err := app.store.MarkTelegramDeliveryFailed(r.Context(), deliveryID, input.Error); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	default:
		writeError(w, http.StatusNotFound, "delivery action not found")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"ok": true})
}

func (app *App) handleListItems(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	includeDeleted := r.URL.Query().Get("include_deleted") == "true"
	since, err := parseTimeQuery(r.URL.Query().Get("since"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid since value, expected RFC3339 timestamp")
		return
	}

	items, err := app.store.ListItems(r.Context(), user.ID, includeDeleted, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{"items": items})
}

func (app *App) handleSyncPull(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	since, err := parseTimeQuery(r.URL.Query().Get("since"))
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid since value, expected RFC3339 timestamp")
		return
	}

	items, err := app.store.ListItems(r.Context(), user.ID, true, since)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"items":       items,
		"server_time": time.Now().UTC(),
	})
}

func (app *App) handleCreateItem(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	var input struct {
		Title             string     `json:"title"`
		Body              string     `json:"body"`
		Lang              string     `json:"lang"`
		Status            string     `json:"status"`
		RemindAt          *time.Time `json:"remind_at"`
		RepeatRule        *string    `json:"repeat_rule"`
		Source            string     `json:"source"`
		DeliverToTelegram bool       `json:"deliver_to_telegram"`
	}

	if !decodeJSON(w, r, &input) {
		return
	}

	if strings.TrimSpace(input.Title) == "" {
		writeError(w, http.StatusBadRequest, "title is required")
		return
	}

	item, err := app.store.CreateItem(r.Context(), database.CreateItemParams{
		UserID:            user.ID,
		Title:             strings.TrimSpace(input.Title),
		Body:              input.Body,
		Lang:              fallbackLang(input.Lang, user.Lang),
		Status:            fallbackValue(input.Status, "active"),
		RemindAt:          input.RemindAt,
		RepeatRule:        input.RepeatRule,
		Source:            fallbackValue(input.Source, "web"),
		DeliverToTelegram: input.DeliverToTelegram,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]any{"item": item})
}

func (app *App) handleItemByID(w http.ResponseWriter, r *http.Request) {
	user := currentUser(r.Context())
	itemID := strings.TrimPrefix(r.URL.Path, "/v1/items/")
	if itemID == "" || strings.Contains(itemID, "/") {
		writeError(w, http.StatusNotFound, "item not found")
		return
	}

	switch r.Method {
	case http.MethodGet:
		item, err := app.store.GetItem(r.Context(), user.ID, itemID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, database.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"item": item})
	case http.MethodPatch:
		var input struct {
			Title             *string    `json:"title"`
			Body              *string    `json:"body"`
			Lang              *string    `json:"lang"`
			Status            *string    `json:"status"`
			RemindAt          *time.Time `json:"remind_at"`
			RepeatRule        *string    `json:"repeat_rule"`
			Source            *string    `json:"source"`
			DeliverToTelegram *bool      `json:"deliver_to_telegram"`
		}

		if !decodeJSON(w, r, &input) {
			return
		}

		current, err := app.store.GetItem(r.Context(), user.ID, itemID)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, database.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}

		item, err := app.store.UpdateItem(r.Context(), database.UpdateItemParams{
			UserID:            user.ID,
			ID:                itemID,
			Title:             chooseString(input.Title, current.Title),
			Body:              chooseString(input.Body, current.Body),
			Lang:              fallbackLang(chooseString(input.Lang, current.Lang), current.Lang),
			Status:            fallbackValue(chooseString(input.Status, current.Status), current.Status),
			RemindAt:          chooseTime(input.RemindAt, current.RemindAt),
			RepeatRule:        chooseStringPointer(input.RepeatRule, current.RepeatRule),
			Source:            fallbackValue(chooseString(input.Source, current.Source), current.Source),
			DeliverToTelegram: chooseBool(input.DeliverToTelegram, current.DeliverToTelegram),
		})
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, database.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"item": item})
	case http.MethodDelete:
		if err := app.store.DeleteItem(r.Context(), user.ID, itemID); err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, database.ErrNotFound) {
				status = http.StatusNotFound
			}
			writeError(w, status, err.Error())
			return
		}

		writeJSON(w, http.StatusOK, map[string]any{"ok": true})
	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}
}

func (app *App) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := bearerToken(r.Header.Get("Authorization"))
		if token == "" {
			writeError(w, http.StatusUnauthorized, "missing bearer token")
			return
		}

		user, err := app.store.FindUserByToken(r.Context(), token)
		if err != nil {
			status := http.StatusInternalServerError
			if errors.Is(err, database.ErrNotFound) {
				status = http.StatusUnauthorized
			}
			writeError(w, status, err.Error())
			return
		}

		next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), userContextKey, user)))
	})
}

func (app *App) requireBotService(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.TrimSpace(r.Header.Get("X-Service-Token")) != app.cfg.BotServiceToken {
			writeError(w, http.StatusUnauthorized, "invalid service token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

func currentUser(ctx context.Context) domain.User {
	user, _ := ctx.Value(userContextKey).(domain.User)
	return user
}

func bearerToken(header string) string {
	token := strings.TrimSpace(header)
	if strings.HasPrefix(strings.ToLower(token), "bearer ") {
		return strings.TrimSpace(token[7:])
	}

	return ""
}

func parseTimeQuery(value string) (*time.Time, error) {
	if strings.TrimSpace(value) == "" {
		return nil, nil
	}

	timestamp, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return nil, err
	}

	return &timestamp, nil
}

func fallbackValue(value, fallback string) string {
	if strings.TrimSpace(value) == "" {
		return fallback
	}

	return strings.TrimSpace(value)
}

func fallbackLang(value, fallback string) string {
	switch strings.TrimSpace(value) {
	case "ru", "en":
		return strings.TrimSpace(value)
	default:
		return fallback
	}
}

func chooseTime(incoming, current *time.Time) *time.Time {
	if incoming != nil {
		return incoming
	}

	return current
}

func chooseStringPointer(incoming, current *string) *string {
	if incoming != nil {
		return incoming
	}

	return current
}

func chooseString(incoming *string, current string) string {
	if incoming != nil {
		return *incoming
	}

	return current
}

func chooseBool(incoming *bool, current bool) bool {
	if incoming != nil {
		return *incoming
	}

	return current
}

func normalizeOptionalString(value *string) *string {
	if value == nil {
		return nil
	}

	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}

	return &trimmed
}

func withCORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	defer r.Body.Close()

	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid JSON body")
		return false
	}

	return true
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]any{
		"error": message,
	})
}

func writeJSON(w http.ResponseWriter, status int, payload any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(payload)
}
