package ipc

// Signal represents the structure that is passed to the service that contains the relevant information
type Signal struct {
	FilingId string `json:"filing_id"`
	UserId   int    `json:"user_id"`
}
