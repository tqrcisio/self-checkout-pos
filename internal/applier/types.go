package applier

import "time"

type Sentinel struct {
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	StartedAt   time.Time `json:"started_at"`
	ExpiresAt   time.Time `json:"expires_at"`
}

type UpdateStatus struct {
	FromVersion string    `json:"from_version"`
	ToVersion   string    `json:"to_version"`
	AppliedAt   time.Time `json:"applied_at,omitempty"`
	AttemptedAt time.Time `json:"attempted_at,omitempty"`
	Status      string    `json:"status"` // "ok" | "rolled_back" | "failed"
	Error       string    `json:"error,omitempty"`
}
