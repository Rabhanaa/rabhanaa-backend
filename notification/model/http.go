package model

import (
	"encoding/json"
	"time"
)

type NotificationResponse struct {
	ID        int32           `json:"id"`
	UserID    int32           `json:"user_id"`
	Title     string          `json:"title"`
	Body      string          `json:"body"`
	EventType string          `json:"event_type"`
	Data      json.RawMessage `json:"data"`
	IsRead    bool            `json:"is_read"`
	CreatedAt time.Time       `json:"created_at"`
}
