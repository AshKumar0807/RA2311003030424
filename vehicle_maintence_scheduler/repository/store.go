package repository

import (
	"crypto/rand"
	"fmt"
	"sync"
	"time"

	logging "github.com/AshKumar0807/RA2311003030424/logging_middleware"
	"github.com/AshKumar0807/RA2311003030424/vehicle_maintence_scheduler/domain"
)

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type Store struct {
	mu       sync.RWMutex
	vehicles map[string]*domain.Vehicle
	jobs     map[string]*domain.MaintenanceJob
	log      *logging.Logger
}

func New(logger *logging.Logger) *Store {
	logger.Info("db", "in-memory vehicle maintenance store initialised")
	return &Store{vehicles: make(map[string]*domain.Vehicle), jobs: make(map[string]*domain.MaintenanceJob), log: logger}
}

func (s *Store) SaveVehicle(req domain.CreateVehicleRequest) (*domain.Vehicle, error) {
	s.log.Debug("repository", fmt.Sprintf("saving vehicle: registration=%s", req.RegistrationNo))
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range s.vehicles {
		if v.RegistrationNo == req.RegistrationNo {
			s.log.Warn("repository", fmt.Sprintf("duplicate registration: %s", req.RegistrationNo))
			return nil, fmt.Errorf("vehicle with registration %q already exists", req.RegistrationNo)
		}
	}
	v := &domain.Vehicle{ID: newUUID(), OwnerName: req.OwnerName, RegistrationNo: req.RegistrationNo, Make: req.Make, Model: req.Model, Year: req.Year, CreatedAt: time.Now().UTC()}
	s.vehicles[v.ID] = v
	s.log.Info("repository", fmt.Sprintf("vehicle persisted: id=%s", v.ID))
	return v, nil
}

func (s *Store) GetVehicle(id string) (*domain.Vehicle, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	v, ok := s.vehicles[id]
	if !ok {
		return nil, fmt.Errorf("vehicle %q not found", id)
	}
	return v, nil
}

func (s *Store) ListVehicles() []*domain.Vehicle {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*domain.Vehicle, 0, len(s.vehicles))
	for _, v := range s.vehicles {
		out = append(out, v)
	}
	return out
}

func (s *Store) SaveJob(req domain.ScheduleJobRequest) (*domain.MaintenanceJob, error) {
	s.log.Debug("repository", fmt.Sprintf("saving job: vehicle_id=%s type=%s", req.VehicleID, req.Type))
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	job := &domain.MaintenanceJob{ID: newUUID(), VehicleID: req.VehicleID, Type: req.Type, Status: domain.StatusScheduled, ScheduledAt: req.ScheduledAt, Notes: req.Notes, CreatedAt: now, UpdatedAt: now}
	s.jobs[job.ID] = job
	s.log.Info("repository", fmt.Sprintf("job persisted: id=%s", job.ID))
	return job, nil
}

func (s *Store) GetJob(id string) (*domain.MaintenanceJob, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q not found", id)
	}
	return job, nil
}

func (s *Store) ListJobsForVehicle(vehicleID string) []*domain.MaintenanceJob {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*domain.MaintenanceJob
	for _, j := range s.jobs {
		if j.VehicleID == vehicleID {
			out = append(out, j)
		}
	}
	return out
}

func (s *Store) UpdateJobStatus(id string, req domain.UpdateJobStatusRequest) (*domain.MaintenanceJob, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	job, ok := s.jobs[id]
	if !ok {
		return nil, fmt.Errorf("job %q not found", id)
	}
	job.Status = req.Status
	job.UpdatedAt = time.Now().UTC()
	if req.TechnicianName != "" {
		job.TechnicianName = req.TechnicianName
	}
	if req.Notes != "" {
		job.Notes = req.Notes
	}
	if req.Status == domain.StatusCompleted {
		now := time.Now().UTC()
		job.CompletedAt = &now
	}
	s.log.Info("repository", fmt.Sprintf("job updated: id=%s status=%s", id, req.Status))
	return job, nil
}
