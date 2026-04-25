package domain

import "time"

type Item struct {
	ID                string     `json:"id"`
	UserID            string     `json:"user_id"`
	Title             string     `json:"title"`
	Body              string     `json:"body"`
	Lang              string     `json:"lang"`
	Status            string     `json:"status"`
	RemindAt          *time.Time `json:"remind_at,omitempty"`
	RepeatRule        *string    `json:"repeat_rule,omitempty"`
	Version           int64      `json:"version"`
	UpdatedAt         time.Time  `json:"updated_at"`
	DeletedAt         *time.Time `json:"deleted_at,omitempty"`
	Source            string     `json:"source"`
	DeliverToTelegram bool       `json:"deliver_to_telegram"`
}

func ItemSchema() map[string]any {
	return map[string]any{
		"name":        "items",
		"description": "Single entity for notes and reminders across web, desktop, and Telegram.",
		"fields": []map[string]string{
			{"name": "title", "type": "string"},
			{"name": "body", "type": "string"},
			{"name": "lang", "type": "string"},
			{"name": "status", "type": "string"},
			{"name": "remind_at", "type": "timestamp|null"},
			{"name": "repeat_rule", "type": "string|null"},
			{"name": "updated_at", "type": "timestamp"},
			{"name": "deleted_at", "type": "timestamp|null"},
			{"name": "version", "type": "int64"},
			{"name": "source", "type": "string"},
		},
	}
}
