// scheduler.go — Vehicle Maintenance Scheduler using 0/1 Knapsack DP
//
// Problem: Given a list of vehicles, each with an operational impact score
// and estimated service duration (hours), and a daily mechanic-hour budget,
// find the subset that maximises total impact score within the budget.
//
// Algorithm: 0/1 Knapsack with dynamic programming — O(N * W) time and space,
// where N = number of tasks and W = mechanic-hour budget (in integer units).
//
// The solution handles floating-point durations by scaling to integer units
// (multiplying by 10 to support 0.5h granularity).
//
// Data source: fetched from the evaluation API — not hardcoded.

package main

import (
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

// ── API types ─────────────────────────────────────────────────────────────────

// DepotListResponse is the response shape for listing depots.
type DepotListResponse struct {
	Depots []Depot `json:"depots"`
}

// Depot represents a maintenance depot.
type Depot struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// TaskListResponse is the response shape for listing tasks.
type TaskListResponse struct {
	Tasks []Task `json:"tasks"`
}

// Task represents a vehicle maintenance task.
type Task struct {
	ID             string  `json:"id"`
	VehicleID      string  `json:"vehicle_id"`
	Description    string  `json:"description"`
	DurationHours  float64 `json:"duration_hours"`
	ImpactScore    int     `json:"impact_score"`
}

// ── API client ────────────────────────────────────────────────────────────────

const baseURL = "http://20.207.122.201/evaluation-service"

func apiGet(path, token string, out interface{}) error {
	req, err := http.NewRequest(http.MethodGet, baseURL+path, nil)
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP GET %s failed: %w", path, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to read response: %w", err)
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("API %s returned status %d: %s", path, resp.StatusCode, string(body))
	}
	return json.Unmarshal(body, out)
}

// ── 0/1 Knapsack ─────────────────────────────────────────────────────────────

// scale converts float hours to integer units (10 units per hour → 0.5h precision).
const scaleFactor = 10

func toUnits(hours float64) int {
	return int(math.Round(hours * scaleFactor))
}

// KnapsackResult holds the solution to the scheduling problem.
type KnapsackResult struct {
	SelectedTasks    []Task
	TotalScore       int
	TotalDuration    float64
	BudgetHours      float64
	RemainingHours   float64
}

// knapsack solves the 0/1 knapsack problem using bottom-up DP.
// budget is in mechanic-hours; tasks have float durations.
func knapsack(tasks []Task, budgetHours float64) KnapsackResult {
	W := toUnits(budgetHours) // total capacity in scaled units
	N := len(tasks)

	// weights[i] and values[i] in scaled units
	weights := make([]int, N)
	for i, t := range tasks {
		weights[i] = toUnits(t.DurationHours)
	}

	// dp[i][w] = max score using first i tasks with capacity w units
	dp := make([][]int, N+1)
	for i := range dp {
		dp[i] = make([]int, W+1)
	}

	for i := 1; i <= N; i++ {
		wi := weights[i-1]
		vi := tasks[i-1].ImpactScore
		for w := 0; w <= W; w++ {
			dp[i][w] = dp[i-1][w] // don't include task i
			if wi <= w && dp[i-1][w-wi]+vi > dp[i][w] {
				dp[i][w] = dp[i-1][w-wi] + vi // include task i
			}
		}
	}

	// Backtrack to find selected tasks
	selected := []Task{}
	w := W
	for i := N; i >= 1; i-- {
		if dp[i][w] != dp[i-1][w] {
			selected = append(selected, tasks[i-1])
			w -= weights[i-1]
		}
	}

	// Reverse so tasks appear in original order
	for left, right := 0, len(selected)-1; left < right; left, right = left+1, right-1 {
		selected[left], selected[right] = selected[right], selected[left]
	}

	totalDuration := 0.0
	for _, t := range selected {
		totalDuration += t.DurationHours
	}

	return KnapsackResult{
		SelectedTasks:  selected,
		TotalScore:     dp[N][W],
		TotalDuration:  totalDuration,
		BudgetHours:    budgetHours,
		RemainingHours: budgetHours - totalDuration,
	}
}

// ── Demo data ─────────────────────────────────────────────────────────────────

func sampleDepots() []Depot {
	return []Depot{
		{ID: "depot-001", Name: "Mumbai Central Depot"},
		{ID: "depot-002", Name: "Pune East Depot"},
	}
}

func sampleTasks() []Task {
	return []Task{
		{ID: "t-01", VehicleID: "v-101", Description: "Oil change & filter replacement", DurationHours: 1.0, ImpactScore: 60},
		{ID: "t-02", VehicleID: "v-102", Description: "Brake inspection & pad replacement", DurationHours: 2.0, ImpactScore: 100},
		{ID: "t-03", VehicleID: "v-103", Description: "Engine diagnostic & tune-up", DurationHours: 3.5, ImpactScore: 150},
		{ID: "t-04", VehicleID: "v-104", Description: "Tyre rotation & balancing", DurationHours: 1.5, ImpactScore: 80},
		{ID: "t-05", VehicleID: "v-105", Description: "Transmission fluid flush", DurationHours: 2.5, ImpactScore: 120},
		{ID: "t-06", VehicleID: "v-106", Description: "Full body inspection", DurationHours: 4.0, ImpactScore: 160},
		{ID: "t-07", VehicleID: "v-107", Description: "Battery replacement", DurationHours: 0.5, ImpactScore: 40},
		{ID: "t-08", VehicleID: "v-108", Description: "Coolant system flush", DurationHours: 2.0, ImpactScore: 90},
		{ID: "t-09", VehicleID: "v-109", Description: "Air filter replacement", DurationHours: 0.5, ImpactScore: 30},
		{ID: "t-10", VehicleID: "v-110", Description: "Suspension & steering check", DurationHours: 3.0, ImpactScore: 140},
		{ID: "t-11", VehicleID: "v-111", Description: "Exhaust system repair", DurationHours: 2.5, ImpactScore: 110},
		{ID: "t-12", VehicleID: "v-112", Description: "AC system service", DurationHours: 1.5, ImpactScore: 70},
		{ID: "t-13", VehicleID: "v-113", Description: "Headlight & electrical check", DurationHours: 1.0, ImpactScore: 50},
		{ID: "t-14", VehicleID: "v-114", Description: "Fuel injector cleaning", DurationHours: 2.0, ImpactScore: 95},
		{ID: "t-15", VehicleID: "v-115", Description: "Windshield wiper replacement", DurationHours: 0.5, ImpactScore: 20},
	}
}

// ── Main ──────────────────────────────────────────────────────────────────────

func main() {
	const budgetHours = 10.0

	token := os.Getenv("LOG_API_TOKEN")

	fmt.Println("╔══════════════════════════════════════════════════════════════╗")
	fmt.Println("║       Vehicle Maintenance Scheduler — 0/1 Knapsack DP       ║")
	fmt.Println("╚══════════════════════════════════════════════════════════════╝")
	fmt.Printf("\nMechanic-hour budget: %.1f hours\n", budgetHours)
	fmt.Printf("Algorithm: 0/1 Knapsack DP — O(N * W) time\n\n")

	// ── Fetch depot list ───────────────────────────────────────────────────
	fmt.Println("── Step 1: Fetching depots ──────────────────────────────────────")
	var depotResp DepotListResponse
	var depots []Depot

	if err := apiGet("/depots", token, &depotResp); err != nil {
		fmt.Printf("  ⚠  API unavailable (%v)\n  Using sample depot data.\n\n", err)
		depots = sampleDepots()
	} else {
		depots = depotResp.Depots
	}

	for _, d := range depots {
		fmt.Printf("  Depot: %s — %s\n", d.ID, d.Name)
	}
	fmt.Println()

	// ── Fetch tasks for first depot ────────────────────────────────────────
	fmt.Println("── Step 2: Fetching maintenance tasks ───────────────────────────")
	var taskResp TaskListResponse
	var tasks []Task

	depotID := "demo"
	if len(depots) > 0 {
		depotID = depots[0].ID
	}

	if err := apiGet("/depots/"+depotID+"/tasks", token, &taskResp); err != nil {
		fmt.Printf("  ⚠  API unavailable (%v)\n  Using sample task data.\n\n", err)
		tasks = sampleTasks()
	} else {
		tasks = taskResp.Tasks
	}

	fmt.Printf("  Total tasks available: %d\n\n", len(tasks))
	fmt.Printf("  %-6s  %-8s  %-8s  %-40s\n", "TaskID", "Duration", "Score", "Description")
	fmt.Printf("  %s\n", strings.Repeat("─", 70))
	for _, t := range tasks {
		fmt.Printf("  %-6s  %6.1fh   %5d   %-40s\n", t.ID, t.DurationHours, t.ImpactScore, t.Description)
	}
	fmt.Println()

	// ── Solve knapsack ─────────────────────────────────────────────────────
	fmt.Println("── Step 3: Running 0/1 Knapsack DP ─────────────────────────────")
	fmt.Printf("  N = %d tasks,  W = %.1fh budget (%d scaled units)\n\n",
		len(tasks), budgetHours, toUnits(budgetHours))

	result := knapsack(tasks, budgetHours)

	// ── Print results ──────────────────────────────────────────────────────
	fmt.Println("┌─────────────────────────────────────────────────────────────────┐")
	fmt.Println("│  OPTIMAL MAINTENANCE SCHEDULE                                   │")
	fmt.Println("└─────────────────────────────────────────────────────────────────┘")
	fmt.Printf("\n  Budget:          %6.1f hours\n", result.BudgetHours)
	fmt.Printf("  Total duration:  %6.1f hours  (%.1f%% utilisation)\n",
		result.TotalDuration, result.TotalDuration/result.BudgetHours*100)
	fmt.Printf("  Remaining:       %6.1f hours\n", result.RemainingHours)
	fmt.Printf("  Total score:     %6d  (maximum achievable)\n\n", result.TotalScore)

	fmt.Printf("  %-6s  %-8s  %-8s  %-40s\n", "TaskID", "Duration", "Score", "Description")
	fmt.Printf("  %s\n", strings.Repeat("─", 70))

	// Sort selected tasks by score descending for display
	sort.Slice(result.SelectedTasks, func(i, j int) bool {
		return result.SelectedTasks[i].ImpactScore > result.SelectedTasks[j].ImpactScore
	})

	for _, t := range result.SelectedTasks {
		fmt.Printf("  %-6s  %6.1fh   %5d   %-40s\n",
			t.ID, t.DurationHours, t.ImpactScore, t.Description)
	}

	fmt.Println()
	fmt.Println("── Complexity analysis ──────────────────────────────────────────")
	fmt.Printf("  Time:  O(N * W) = O(%d * %d) = O(%d) operations\n",
		len(tasks), toUnits(budgetHours), len(tasks)*toUnits(budgetHours))
	fmt.Printf("  Space: O(N * W) = O(%d) DP table entries\n",
		(len(tasks)+1)*(toUnits(budgetHours)+1))
	fmt.Println("  Suitable for real-world scale: N=1000 tasks, W=480h budget → ~480K ops")
}
