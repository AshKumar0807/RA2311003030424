package service

import (
	"fmt"
	"time"

	logging "github.com/AshKumar0807/RA2311003030424/logging_middleware"
	"github.com/AshKumar0807/RA2311003030424/vehicle_maintence_scheduler/domain"
	"github.com/AshKumar0807/RA2311003030424/vehicle_maintence_scheduler/repository"
)

type Service struct {
	store *repository.Store
	log   *logging.Logger
}

func New(store *repository.Store, logger *logging.Logger) *Service {
	logger.Info("service", "vehicle maintenance service initialised")
	return &Service{store: store, log: logger}
}

func (s *Service) RegisterVehicle(req domain.CreateVehicleRequest) (*domain.Vehicle, error) {
	s.log.Info("service", fmt.Sprintf("registering vehicle: owner=%s registration=%s", req.OwnerName, req.RegistrationNo))
	if err := s.validateVehicleRequest(req); err != nil {
		s.log.Error("service", fmt.Sprintf("vehicle registration validation failed: %v", err))
		return nil, err
	}
	vehicle, err := s.store.SaveVehicle(req)
	if err != nil {
		s.log.Error("service", fmt.Sprintf("failed to persist vehicle registration=%s: %v", req.RegistrationNo, err))
		return nil, err
	}
	s.log.Info("service", fmt.Sprintf("vehicle registered successfully: id=%s registration=%s", vehicle.ID, vehicle.RegistrationNo))
	return vehicle, nil
}

func (s *Service) GetVehicle(id string) (*domain.Vehicle, error) {
	s.log.Debug("service", fmt.Sprintf("fetching vehicle details: id=%s", id))
	vehicle, err := s.store.GetVehicle(id)
	if err != nil {
		s.log.Warn("service", fmt.Sprintf("vehicle lookup failed: id=%s err=%v", id, err))
		return nil, err
	}
	return vehicle, nil
}

func (s *Service) ListVehicles() []*domain.Vehicle {
	s.log.Debug("service", "listing all registered vehicles")
	return s.store.ListVehicles()
}

func (s *Service) ScheduleJob(req domain.ScheduleJobRequest) (*domain.MaintenanceJob, error) {
	s.log.Info("service", fmt.Sprintf("scheduling maintenance job: vehicle_id=%s type=%s scheduled_at=%s", req.VehicleID, req.Type, req.ScheduledAt.Format(time.RFC3339)))
	if _, err := s.store.GetVehicle(req.VehicleID); err != nil {
		s.log.Error("service", fmt.Sprintf("cannot schedule job — vehicle not found: vehicle_id=%s", req.VehicleID))
		return nil, fmt.Errorf("cannot schedule job: %w", err)
	}
	if err := s.validateJobRequest(req); err != nil {
		s.log.Error("service", fmt.Sprintf("job scheduling validation failed: %v", err))
		return nil, err
	}
	job, err := s.store.SaveJob(req)
	if err != nil {
		s.log.Error("service", fmt.Sprintf("failed to persist maintenance job: vehicle_id=%s err=%v", req.VehicleID, err))
		return nil, err
	}
	s.log.Info("service", fmt.Sprintf("maintenance job scheduled: id=%s vehicle_id=%s type=%s", job.ID, job.VehicleID, job.Type))
	return job, nil
}

func (s *Service) GetJob(id string) (*domain.MaintenanceJob, error) {
	s.log.Debug("service", fmt.Sprintf("fetching maintenance job: id=%s", id))
	return s.store.GetJob(id)
}

func (s *Service) ListJobsForVehicle(vehicleID string) []*domain.MaintenanceJob {
	s.log.Debug("service", fmt.Sprintf("listing maintenance jobs for vehicle_id=%s", vehicleID))
	return s.store.ListJobsForVehicle(vehicleID)
}

func (s *Service) UpdateJobStatus(id string, req domain.UpdateJobStatusRequest) (*domain.MaintenanceJob, error) {
	s.log.Info("service", fmt.Sprintf("updating job status: id=%s new_status=%s technician=%s", id, req.Status, req.TechnicianName))
	job, err := s.store.GetJob(id)
	if err != nil {
		s.log.Warn("service", fmt.Sprintf("update requested for non-existent job: id=%s", id))
		return nil, err
	}
	if err := s.validateStatusTransition(job.Status, req.Status); err != nil {
		s.log.Warn("service", fmt.Sprintf("invalid status transition: id=%s %s → %s: %v", id, job.Status, req.Status, err))
		return nil, err
	}
	updated, err := s.store.UpdateJobStatus(id, req)
	if err != nil {
		s.log.Error("service", fmt.Sprintf("failed to update job status: id=%s err=%v", id, err))
		return nil, err
	}
	s.log.Info("service", fmt.Sprintf("job status updated: id=%s status=%s", updated.ID, updated.Status))
	return updated, nil
}

func (s *Service) validateVehicleRequest(req domain.CreateVehicleRequest) error {
	if req.OwnerName == "" {
		return fmt.Errorf("owner_name is required")
	}
	if req.RegistrationNo == "" {
		return fmt.Errorf("registration_no is required")
	}
	if req.Make == "" {
		return fmt.Errorf("make is required")
	}
	if req.Model == "" {
		return fmt.Errorf("model is required")
	}
	if req.Year < 1900 || req.Year > time.Now().Year()+1 {
		return fmt.Errorf("year %d is out of valid range", req.Year)
	}
	return nil
}

func (s *Service) validateJobRequest(req domain.ScheduleJobRequest) error {
	valid := map[domain.MaintenanceType]bool{
		domain.TypeOilChange: true, domain.TypeTyreRotation: true,
		domain.TypeBrakeInspect: true, domain.TypeGeneralService: true, domain.TypeEngineCheck: true,
	}
	if !valid[req.Type] {
		return fmt.Errorf("unknown maintenance type: %q", req.Type)
	}
	if req.ScheduledAt.Before(time.Now()) {
		return fmt.Errorf("scheduled_at must be a future time")
	}
	return nil
}

func (s *Service) validateStatusTransition(from, to domain.MaintenanceStatus) error {
	allowed := map[domain.MaintenanceStatus][]domain.MaintenanceStatus{
		domain.StatusPending:    {domain.StatusScheduled, domain.StatusCancelled},
		domain.StatusScheduled:  {domain.StatusInProgress, domain.StatusCancelled},
		domain.StatusInProgress: {domain.StatusCompleted, domain.StatusCancelled},
	}
	for _, valid := range allowed[from] {
		if valid == to {
			return nil
		}
	}
	return fmt.Errorf("cannot transition job from %q to %q", from, to)
}
