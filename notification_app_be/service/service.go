package service

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	logging "github.com/AshKumar0807/RA2311003030424/logging_middleware"
	"github.com/AshKumar0807/RA2311003030424/notification_app_be/domain"
)

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

const maxRetries = 3

type NotificationService struct {
	mu            sync.RWMutex
	notifications map[string]*domain.Notification
	log           *logging.Logger
}

func New(logger *logging.Logger) *NotificationService {
	logger.Info("service", "notification service initialised")
	return &NotificationService{notifications: make(map[string]*domain.Notification), log: logger}
}

func (s *NotificationService) Send(req domain.SendNotificationRequest) (*domain.Notification, error) {
	s.log.Info("service", fmt.Sprintf("dispatching notification: job_id=%s channel=%s", req.JobID, req.Channel))
	if req.JobID == "" {
		return nil, fmt.Errorf("job_id required")
	}
	if req.RecipientID == "" {
		return nil, fmt.Errorf("recipient_id required")
	}
	if req.Body == "" {
		return nil, fmt.Errorf("body required")
	}
	valid := map[domain.Channel]bool{domain.ChannelEmail: true, domain.ChannelSMS: true, domain.ChannelPush: true}
	if !valid[req.Channel] {
		return nil, fmt.Errorf("channel must be email, sms, or push")
	}

	now := time.Now().UTC()
	n := &domain.Notification{ID: newUUID(), JobID: req.JobID, RecipientID: req.RecipientID, Channel: req.Channel, Subject: req.Subject, Body: req.Body, Status: domain.StatusSent, CreatedAt: now, UpdatedAt: now}
	sentAt := now
	n.SentAt = &sentAt
	s.mu.Lock()
	s.notifications[n.ID] = n
	s.mu.Unlock()
	s.log.Info("service", fmt.Sprintf("notification sent: id=%s", n.ID))
	return n, nil
}

func (s *NotificationService) GetNotification(id string) (*domain.Notification, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n, ok := s.notifications[id]
	if !ok {
		return nil, fmt.Errorf("notification %q not found", id)
	}
	return n, nil
}

func (s *NotificationService) ListNotifications() []*domain.Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Notification, 0, len(s.notifications))
	for _, n := range s.notifications {
		out = append(out, n)
	}
	return out
}
