package domain

import "time"

type Channel string
const ( ChannelEmail Channel = "email"; ChannelSMS Channel = "sms"; ChannelPush Channel = "push" )

type NotificationStatus string
const (
	StatusPending  NotificationStatus = "pending"
	StatusSent     NotificationStatus = "sent"
	StatusFailed   NotificationStatus = "failed"
	StatusRetrying NotificationStatus = "retrying"
	StatusDead     NotificationStatus = "dead"
)

type Notification struct {
	ID          string             `json:"id"`
	JobID       string             `json:"job_id"`
	RecipientID string             `json:"recipient_id"`
	Channel     Channel            `json:"channel"`
	Subject     string             `json:"subject,omitempty"`
	Body        string             `json:"body"`
	Status      NotificationStatus `json:"status"`
	Attempts    int                `json:"attempts"`
	LastError   string             `json:"last_error,omitempty"`
	SentAt      *time.Time         `json:"sent_at,omitempty"`
	CreatedAt   time.Time          `json:"created_at"`
	UpdatedAt   time.Time          `json:"updated_at"`
}

type SendNotificationRequest struct {
	JobID       string  `json:"job_id"`
	RecipientID string  `json:"recipient_id"`
	Channel     Channel `json:"channel"`
	Subject     string  `json:"subject,omitempty"`
	Body        string  `json:"body"`
}
