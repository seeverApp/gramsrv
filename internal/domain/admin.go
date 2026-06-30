package domain

import "time"

type AdminCommandStatus string

const (
	AdminCommandRunning   AdminCommandStatus = "running"
	AdminCommandCompleted AdminCommandStatus = "completed"
	AdminCommandFailed    AdminCommandStatus = "failed"
)

type AdminCommand struct {
	CommandID    string
	Actor        string
	Action       string
	TargetUserID int64
	TargetPeer   Peer
	DryRun       bool
	Reason       string
	RequestJSON  []byte
	ResultJSON   []byte
	Status       AdminCommandStatus
	Error        string
	CreatedAt    time.Time
	CompletedAt  *time.Time
}

type AccountSendRestriction struct {
	UserID    int64
	Frozen    bool
	Reason    string
	Actor     string
	CommandID string
	UpdatedAt time.Time
}
