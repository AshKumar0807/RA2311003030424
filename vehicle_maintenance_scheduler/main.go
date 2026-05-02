// Vehicle Maintenance Scheduler — HTTP server (stdlib net/http, no external deps)
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	logging "github.com/student/ROLL_NUMBER/logging_middleware"
	"github.com/student/ROLL_NUMBER/vehicle_maintenance_scheduler/config"
	"github.com/student/ROLL_NUMBER/vehicle_maintenance_scheduler/domain"
	"github.com/student/ROLL_NUMBER/vehicle_maintenance_scheduler/repository"
	"github.com/student/ROLL_NUMBER/vehicle_maintenance_scheduler/service"
)

var (
	logger *logging.Logger
	svc    *service.Service
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// Router
func router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	logger.Info("middleware", fmt.Sprintf("→ %s %s client=%s", r.Method, path, r.RemoteAddr))

	switch {
	case path == "/health":
		writeJSON(w, 200, map[string]string{"status": "ok", "service": "vehicle-maintenance-scheduler"})

	case path == "/api/v1/vehicles" && r.Method == http.MethodPost:
		createVehicle(w, r)
	case path == "/api/v1/vehicles" && r.Method == http.MethodGet:
		listVehicles(w, r)
	case strings.HasPrefix(path, "/api/v1/vehicles/") && strings.HasSuffix(path, "/jobs") && r.Method == http.MethodGet:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/vehicles/"), "/jobs")
		listJobs(w, r, id)
	case strings.HasPrefix(path, "/api/v1/vehicles/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(path, "/api/v1/vehicles/")
		getVehicle(w, r, id)

	case path == "/api/v1/jobs" && r.Method == http.MethodPost:
		createJob(w, r)
	case strings.HasSuffix(path, "/status") && r.Method == http.MethodPatch:
		id := strings.TrimSuffix(strings.TrimPrefix(path, "/api/v1/jobs/"), "/status")
		updateJobStatus(w, r, id)
	case strings.HasPrefix(path, "/api/v1/jobs/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(path, "/api/v1/jobs/")
		getJob(w, r, id)

	default:
		errJSON(w, 404, "not found")
	}
}

func createVehicle(w http.ResponseWriter, r *http.Request) {
	var req domain.CreateVehicleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid JSON: "+err.Error()); return
	}
	v, err := svc.RegisterVehicle(req)
	if err != nil {
		errJSON(w, 422, err.Error()); return
	}
	writeJSON(w, 201, v)
}

func listVehicles(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, 200, svc.ListVehicles())
}

func getVehicle(w http.ResponseWriter, r *http.Request, id string) {
	v, err := svc.GetVehicle(id)
	if err != nil { errJSON(w, 404, err.Error()); return }
	writeJSON(w, 200, v)
}

func listJobs(w http.ResponseWriter, r *http.Request, vehicleID string) {
	writeJSON(w, 200, svc.ListJobsForVehicle(vehicleID))
}

func createJob(w http.ResponseWriter, r *http.Request) {
	var raw struct {
		VehicleID   string `json:"vehicle_id"`
		Type        string `json:"type"`
		ScheduledAt string `json:"scheduled_at"`
		Notes       string `json:"notes"`
	}
	if err := json.NewDecoder(r.Body).Decode(&raw); err != nil {
		errJSON(w, 400, "invalid JSON: "+err.Error()); return
	}
	req, err := domain.ParseScheduleJobRequest(raw.VehicleID, raw.Type, raw.ScheduledAt, raw.Notes)
	if err != nil { errJSON(w, 400, err.Error()); return }
	job, err := svc.ScheduleJob(req)
	if err != nil { errJSON(w, 422, err.Error()); return }
	writeJSON(w, 201, job)
}

func getJob(w http.ResponseWriter, r *http.Request, id string) {
	job, err := svc.GetJob(id)
	if err != nil { errJSON(w, 404, err.Error()); return }
	writeJSON(w, 200, job)
}

func updateJobStatus(w http.ResponseWriter, r *http.Request, id string) {
	var req domain.UpdateJobStatusRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		errJSON(w, 400, "invalid JSON: "+err.Error()); return
	}
	job, err := svc.UpdateJobStatus(id, req)
	if err != nil { errJSON(w, 422, err.Error()); return }
	writeJSON(w, 200, job)
}

func main() {
	logger = logging.New(os.Getenv("LOG_API_TOKEN"))
	logger.Info("config", "vehicle maintenance scheduler starting up")

	cfg, err := config.Load(logger)
	if err != nil {
		logger.Fatal("config", fmt.Sprintf("failed to load config: %v", err))
		os.Exit(1)
	}

	store := repository.New(logger)
	svc = service.New(store, logger)

	addr := ":" + cfg.Port
	logger.Info("handler", fmt.Sprintf("HTTP server listening on %s", addr))
	fmt.Printf("Vehicle Maintenance Scheduler running on http://localhost%s\n", addr)

	if err := http.ListenAndServe(addr, http.HandlerFunc(router)); err != nil {
		log.Fatal(err)
	}
}
