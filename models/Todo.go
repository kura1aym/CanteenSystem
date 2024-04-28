package models

type Todo struct {
	ID     int    `json:"ID"`
	Text   string `json:"Text"`
	Done   bool   `json:"Done"`
	UserID int    `json:"UserID"`
}
