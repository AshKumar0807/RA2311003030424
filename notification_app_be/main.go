// Notification App Backend — stdlib net/http server
package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	logging "github.com/student/ROLL_NUMBER/logging_middleware"
	"github.com/student/ROLL_NUMBER/notification_app_be/domain"
	"github.com/student/ROLL_NUMBER/notification_app_be/service"
)

var (
	logger *logging.Logger
	svc    *service.NotificationService
)

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errJSON(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func router(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path
	logger.Info("middleware", fmt.Sprintf("→ %s %s", r.Method, path))

	switch {
	case path == "/health":
		writeJSON(w, 200, map[string]string{"status": "ok", "service": "notification-app-be"})

	case path == "/api/v1/notifications" && r.Method == http.MethodPost:
		var req domain.SendNotificationRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil { errJSON(w, 400, err.Error()); return }
		n, err := svc.Send(req)
		if err != nil { errJSON(w, 422, err.Error()); return }
		writeJSON(w, 201, n)

	case path == "/api/v1/notifications" && r.Method == http.MethodGet:
		writeJSON(w, 200, svc.ListNotifications())

	case strings.HasPrefix(path, "/api/v1/notifications/") && r.Method == http.MethodGet:
		id := strings.TrimPrefix(path, "/api/v1/notifications/")
		n, err := svc.GetNotification(id)
		if err != nil { errJSON(w, 404, err.Error()); return }
		writeJSON(w, 200, n)

	default:
		errJSON(w, 404, "not found")
	}
}

func main() {
	logger = logging.New(os.Getenv("LOG_API_TOKEN"))
	logger.Info("config", "notification app backend starting up")

	port := os.Getenv("PORT")
	if port == "" { port = "8081" }
	svc = service.New(logger)

	addr := ":" + port
	logger.Info("handler", fmt.Sprintf("notification service listening on %s", addr))
	fmt.Printf("Notification App Backend running on http://localhost%s\n", addr)

	if err := http.ListenAndServe(addr, http.HandlerFunc(router)); err != nil {
		log.Fatal(err)
	}
}
