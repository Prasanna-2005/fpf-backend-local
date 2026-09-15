package ipc

import "time"

// Signal represents the structure that is passed to the service that contains the relevant information
type Signal struct {
	FilingId    string    `json:"filing_id"`
	UserId      string    `json:"user_id"`
	Series      string    `json:"series"`
	Timestamp   time.Time `json:"timestamp"`
	Replacement bool      `json:"replacement"`
}
