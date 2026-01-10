package model

import "time"

type Notification struct {
	ID        int64     `json:"id"`
	Room      string    `json:"room"`
	Type      string    `json:"type"`
	Title     string    `json:"title"`
	Body      string    `json:"body"`
	CreatedAt time.Time `json:"created_at"`
}
