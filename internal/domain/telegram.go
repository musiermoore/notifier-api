package domain

import "time"

type TelegramLinkCode struct {
	Code      string     `json:"code"`
	UserID    string     `json:"user_id"`
	ExpiresAt time.Time  `json:"expires_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}
