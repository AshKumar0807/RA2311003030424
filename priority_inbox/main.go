// priority_inbox.go — Stage 6: Priority Inbox using a Min-Heap
//
// Fetches notifications from the evaluation API, scores each one using
// a combination of type weight and recency, then uses a fixed-size min-heap
// to efficiently compute the top-N (default 10) most important notifications.
//
// Scoring formula:
//   type_weight:   Placement=300, Result=200, Event=100
//   recency_score: max(0, 100 - floor(hours_since_notification))
//   final_score:   type_weight + recency_score
//
// The min-heap approach processes each notification in O(log N) time,
// making it efficient even as new notifications stream in continuously.

package main

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strings"
	"time"
)

// ── API types ─────────────────────────────────────────────────────────────────

// APIResponse is the shape returned by the evaluation notifications endpoint.
type APIResponse struct {
	Notifications []RawNotification `json:"notifications"`
}

// RawNotification is the raw shape from the API.
type RawNotification struct {
	ID        string `json:"ID"`
	Type      string `json:"Type"`      // "Placement" | "Result" | "Event"
	Message   string `json:"Message"`
	Timestamp string `json:"Timestamp"` // "2026-04-22 17:51:18"
}

// ScoredNotification attaches the computed priority score to a notification.
type ScoredNotification struct {
	ID            string
	Type          string
	Message       string
	Timestamp     time.Time
	TypeWeight    int
	RecencyScore  int
	PriorityScore int
}

// ── Scoring ───────────────────────────────────────────────────────────────────

const (
	weightPlacement = 300
	weightResult    = 200
	weightEvent     = 100
	maxRecency      = 100 // notifications older than 100h contribute 0 from recency
)

func typeWeight(notifType string) int {
	switch strings.ToLower(notifType) {
	case "placement":
		return weightPlacement
	case "result":
		return weightResult
	case "event":
		return weightEvent
	default:
		return 0
	}
}

func recencyScore(t time.Time) int {
	hoursSince := time.Since(t).Hours()
	score := maxRecency - int(math.Floor(hoursSince))
	if score < 0 {
		return 0
	}
	return score
}

func score(n RawNotification) (ScoredNotification, error) {
	// Parse timestamp — API returns "2026-04-22 17:51:18" (no timezone, assume UTC)
	ts, err := time.Parse("2006-01-02 15:04:05", n.Timestamp)
	if err != nil {
		return ScoredNotification{}, fmt.Errorf("bad timestamp %q: %w", n.Timestamp, err)
	}
	ts = ts.UTC()

	tw := typeWeight(n.Type)
	rs := recencyScore(ts)

	return ScoredNotification{
		ID:            n.ID,
		Type:          n.Type,
		Message:       n.Message,
		Timestamp:     ts,
		TypeWeight:    tw,
		RecencyScore:  rs,
		PriorityScore: tw + rs,
	}, nil
}

// ── Min-Heap ──────────────────────────────────────────────────────────────────
// We use a min-heap of size N. For each incoming notification:
//   - If heap size < N: push it.
//   - If heap size == N and new score > heap min: pop min, push new.
//   - Otherwise: discard.
// This gives O(log N) per notification, O(M log N) total for M notifications.

type minHeap []ScoredNotification

func (h minHeap) Len() int            { return len(h) }
func (h minHeap) Less(i, j int) bool  { return h[i].PriorityScore < h[j].PriorityScore }
func (h minHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *minHeap) Push(x interface{}) { *h = append(*h, x.(ScoredNotification)) }
func (h *minHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}

// topN returns the top-n notifications by priority score using a min-heap.
// Result is sorted highest-score first.
func topN(notifications []ScoredNotification, n int) []ScoredNotification {
	h := &minHeap{}
	heap.Init(h)

	for _, notif := range notifications {
		if h.Len() < n {
			heap.Push(h, notif)
		} else if notif.PriorityScore > (*h)[0].PriorityScore {
			heap.Pop(h)
			heap.Push(h, notif)
		}
	}

	// Extract and reverse so highest score is first
	result := make([]ScoredNotification, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(ScoredNotification)
	}
	return result
}

// ── API fetch ─────────────────────────────────────────────────────────────────

func fetchNotifications(apiURL, token string) ([]RawNotification, error) {
	req, err := http.NewRequest(http.MethodGet, apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	// Try array first, then wrapped object
	var raw []RawNotification
	if err := json.Unmarshal(body, &raw); err == nil {
		return raw, nil
	}

	var wrapped APIResponse
	if err := json.Unmarshal(body, &wrapped); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w\nbody: %s", err, string(body))
	}
	return wrapped.Notifications, nil
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	const (
		apiURL  = "http://20.207.122.201/evaluation-service/notifications"
		topSize = 10
	)

	token := os.Getenv("LOG_API_TOKEN")
	if token == "" {
		fmt.Fprintln(os.Stderr, "warning: LOG_API_TOKEN not set — request may be rejected by the protected API")
	}

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║         Campus Notification Platform — Priority Inbox        ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nFetching notifications from: %s\n\n", apiURL)

	rawNotifications, err := fetchNotifications(apiURL, token)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error fetching notifications: %v\n", err)
		// Use mock data so the scoring / heap logic can still be demonstrated
		fmt.Println("⚠  Could not reach API — running with sample data for demonstration.\n")
		rawNotifications = sampleNotifications()
	}

	fmt.Printf("Total notifications fetched: %d\n\n", len(rawNotifications))

	// Score all notifications
	var scored []ScoredNotification
	var parseErrors int
	for _, n := range rawNotifications {
		s, err := score(n)
		if err != nil {
			parseErrors++
			fmt.Fprintf(os.Stderr, "  skip %s: %v\n", n.ID, err)
			continue
		}
		scored = append(scored, s)
	}
	if parseErrors > 0 {
		fmt.Printf("Skipped %d notification(s) due to parse errors.\n\n", parseErrors)
	}

	// Compute top-N using min-heap
	top := topN(scored, topSize)

	// ── Print results ──────────────────────────────────────────────────────
	fmt.Printf("┌─────────────────────────────────────────────────────────────────┐\n")
	fmt.Printf("│  TOP %d PRIORITY NOTIFICATIONS                                   │\n", topSize)
	fmt.Printf("│  Score = TypeWeight + RecencyScore                              │\n")
	fmt.Printf("│  Weights: Placement=300  Result=200  Event=100                  │\n")
	fmt.Printf("│  Recency: max(0, 100 - floor(hours_since_notification))         │\n")
	fmt.Printf("└─────────────────────────────────────────────────────────────────┘\n\n")

	if len(top) == 0 {
		fmt.Println("No notifications to display.")
		return
	}

	for i, n := range top {
		age := time.Since(n.Timestamp)
		ageStr := fmt.Sprintf("%.1fh ago", age.Hours())

		fmt.Printf("  #%02d  %-12s  Score: %3d  (type=%3d + recency=%3d)\n",
			i+1, n.Type, n.PriorityScore, n.TypeWeight, n.RecencyScore)
		fmt.Printf("       ID:      %s\n", n.ID)
		fmt.Printf("       Message: %s\n", n.Message)
		fmt.Printf("       Time:    %s  (%s)\n", n.Timestamp.Format("2006-01-02 15:04:05 UTC"), ageStr)
		fmt.Println()
	}

	fmt.Println("── Scoring summary ──────────────────────────────────────────────")
	placementCount, resultCount, eventCount := 0, 0, 0
	for _, n := range scored {
		switch strings.ToLower(n.Type) {
		case "placement":
			placementCount++
		case "result":
			resultCount++
		case "event":
			eventCount++
		}
	}
	fmt.Printf("  Placement: %d   Result: %d   Event: %d   Total scored: %d\n",
		placementCount, resultCount, eventCount, len(scored))
	fmt.Println()
	fmt.Println("── Heap algorithm ───────────────────────────────────────────────")
	fmt.Printf("  Min-heap capacity: %d\n", topSize)
	fmt.Printf("  Notifications processed: %d\n", len(scored))
	fmt.Printf("  Time complexity: O(M log N) where M=%d, N=%d\n", len(scored), topSize)
	fmt.Println("  Each new notification: O(log N) insertion — efficient for live streams")
}

// sampleNotifications provides realistic mock data matching the API response
// structure seen in the screenshots, so the heap logic can be demonstrated
// even when the protected API is unreachable.
func sampleNotifications() []RawNotification {
	now := time.Now().UTC()
	format := "2006-01-02 15:04:05"
	return []RawNotification{
		{ID: "b283218f-ea5a-4b7c-93a9-1f2f240d64b0", Type: "Placement", Message: "CSX Corporation hiring", Timestamp: now.Add(-5 * time.Minute).Format(format)},
		{ID: "8a7412bd-6065-4d09-8501-a37f11cc848b", Type: "Placement", Message: "Advanced Micro Devices Inc. hiring", Timestamp: now.Add(-15 * time.Minute).Format(format)},
		{ID: "0005513a-142b-4bbc-8678-eefec65e1ede", Type: "Result", Message: "mid-sem", Timestamp: now.Add(-30 * time.Minute).Format(format)},
		{ID: "ea836726-c25e-4f21-a72f-544a6af8a37f", Type: "Result", Message: "project-review", Timestamp: now.Add(-52 * time.Minute).Format(format)},
		{ID: "003cb427-8fc6-47f7-bb00-be228f6b0d2c", Type: "Result", Message: "external", Timestamp: now.Add(-64 * time.Minute).Format(format)},
		{ID: "e5c4ff20-31bf-4d40-8f02-72fda59e8918", Type: "Result", Message: "project-review", Timestamp: now.Add(-78 * time.Minute).Format(format)},
		{ID: "1cfce5ee-ad37-4894-8946-d707627176a5", Type: "Event", Message: "tech-fest", Timestamp: now.Add(-90 * time.Minute).Format(format)},
		{ID: "81589ada-0ad3-4f77-9554-f52fb558e09d", Type: "Event", Message: "farewell", Timestamp: now.Add(-2 * time.Hour).Format(format)},
		{ID: "cf2885a6-45ac-4ba0-b548-6e9e9d4c52c8", Type: "Result", Message: "project-review", Timestamp: now.Add(-3 * time.Hour).Format(format)},
		{ID: "d1a2b3c4-0001-0001-0001-000000000001", Type: "Placement", Message: "Google hiring — software engineer", Timestamp: now.Add(-10 * time.Minute).Format(format)},
		{ID: "d1a2b3c4-0001-0001-0001-000000000002", Type: "Placement", Message: "Microsoft hiring — backend engineer", Timestamp: now.Add(-20 * time.Minute).Format(format)},
		{ID: "d1a2b3c4-0001-0001-0001-000000000003", Type: "Event", Message: "annual sports day", Timestamp: now.Add(-48 * time.Hour).Format(format)},
		{ID: "d1a2b3c4-0001-0001-0001-000000000004", Type: "Result", Message: "end-semester results published", Timestamp: now.Add(-1 * time.Hour).Format(format)},
		{ID: "d1a2b3c4-0001-0001-0001-000000000005", Type: "Event", Message: "cultural fest", Timestamp: now.Add(-72 * time.Hour).Format(format)},
	}
}
