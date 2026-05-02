package domain

import (
	"fmt"
	"time"
)

type MaintenanceStatus string
const (
	StatusPending    MaintenanceStatus = "pending"
	StatusScheduled  MaintenanceStatus = "scheduled"
	StatusInProgress MaintenanceStatus = "in_progress"
	StatusCompleted  MaintenanceStatus = "completed"
	StatusCancelled  MaintenanceStatus = "cancelled"
)

type MaintenanceType string
const (
	TypeOilChange      MaintenanceType = "oil_change"
	TypeTyreRotation   MaintenanceType = "tyre_rotation"
	TypeBrakeInspect   MaintenanceType = "brake_inspection"
	TypeGeneralService MaintenanceType = "general_service"
	TypeEngineCheck    MaintenanceType = "engine_check"
)

type Vehicle struct {
	ID             string    `json:"id"`
	OwnerName      string    `json:"owner_name"`
	RegistrationNo string    `json:"registration_no"`
	Make           string    `json:"make"`
	Model          string    `json:"model"`
	Year           int       `json:"year"`
	CreatedAt      time.Time `json:"created_at"`
}

type MaintenanceJob struct {
	ID             string            `json:"id"`
	VehicleID      string            `json:"vehicle_id"`
	Type           MaintenanceType   `json:"type"`
	Status         MaintenanceStatus `json:"status"`
	ScheduledAt    time.Time         `json:"scheduled_at"`
	CompletedAt    *time.Time        `json:"completed_at,omitempty"`
	Notes          string            `json:"notes,omitempty"`
	TechnicianName string            `json:"technician_name,omitempty"`
	CreatedAt      time.Time         `json:"created_at"`
	UpdatedAt      time.Time         `json:"updated_at"`
}

type CreateVehicleRequest struct {
	OwnerName      string `json:"owner_name"`
	RegistrationNo string `json:"registration_no"`
	Make           string `json:"make"`
	Model          string `json:"model"`
	Year           int    `json:"year"`
}

type ScheduleJobRequest struct {
	VehicleID   string          `json:"vehicle_id"`
	Type        MaintenanceType `json:"type"`
	ScheduledAt time.Time       `json:"scheduled_at"`
	Notes       string          `json:"notes,omitempty"`
}

type UpdateJobStatusRequest struct {
	Status         MaintenanceStatus `json:"status"`
	TechnicianName string            `json:"technician_name,omitempty"`
	Notes          string            `json:"notes,omitempty"`
}

func ParseScheduleJobRequest(vehicleID, jobType, scheduledAt, notes string) (ScheduleJobRequest, error) {
	t, err := time.Parse(time.RFC3339, scheduledAt)
	if err != nil {
		return ScheduleJobRequest{}, fmt.Errorf("scheduled_at must be RFC3339, e.g. 2026-06-01T09:00:00Z")
	}
	return ScheduleJobRequest{
		VehicleID:   vehicleID,
		Type:        MaintenanceType(jobType),
		ScheduledAt: t,
		Notes:       notes,
	}, nil
}
