package domain

import "time"

type TelegramDelivery struct {
	ID        string     `json:"id"`
	ItemID    string     `json:"item_id"`
	UserID    string     `json:"user_id"`
	ChatID    string     `json:"chat_id"`
	Message   string     `json:"message"`
	DeliverAt time.Time  `json:"deliver_at"`
	Status    string     `json:"status"`
	LastError *string    `json:"last_error,omitempty"`
	SentAt    *time.Time `json:"sent_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}
