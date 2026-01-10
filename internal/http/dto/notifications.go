package dto

type CreateNotificationRequest struct {
	Room  string `json:"room"`
	Type  string `json:"type"`
	Title string `json:"title"`
	Body  string `json:"body"`
}
