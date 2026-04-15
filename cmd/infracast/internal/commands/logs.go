package commands

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"text/tabwriter"
	"time"

	"github.com/fatih/color"
	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"

	"github.com/DaviRain-Su/infracast/internal/state"
)

// newLogsCommand creates the logs command
func newLogsCommand() *cobra.Command {
	var (
		env     string
		action  string
		level   string
		limit   int
		since   string
		traceID string
		format  string
		output  string
	)

	cmd := &cobra.Command{
		Use:   "logs",
		Short: "View audit logs and deployment history",
		Long: `View audit logs for deployments, provisioning, and other operations.

Examples:
  # View recent logs
  infracast logs

  # View logs for specific environment
  infracast logs --env production

  # View only deployment logs
  infracast logs --action deploy

  # View error logs only
  infracast logs --level ERROR

  # View logs from last 24 hours
  infracast logs --since 24h

  # View last 50 entries
  infracast logs --limit 50

  # Trace a specific deploy run
  infracast logs --trace trc_1234567890

  # Output as JSON (for scripting)
  infracast logs --format json

  # Wide output with full trace IDs and longer messages
  infracast logs --output wide`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(LogsOptions{
				Env:     env,
				Action:  action,
				Level:   level,
				Limit:   limit,
				Since:   since,
				TraceID: traceID,
				Format:  format,
				Output:  output,
			})
		},
	}

	cmd.Flags().StringVarP(&env, "env", "e", "", "Filter by environment")
	cmd.Flags().StringVarP(&action, "action", "a", "", "Filter by action (deploy, provision, destroy, etc.)")
	cmd.Flags().StringVarP(&level, "level", "l", "", "Filter by level (INFO, WARN, ERROR)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of entries to show")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since duration (e.g., 1h, 24h, 7d)")
	cmd.Flags().StringVar(&traceID, "trace", "", "Filter by trace ID (e.g., trc_1234567890)")
	cmd.Flags().StringVarP(&format, "format", "f", "table", "Output format: table, json")
	cmd.Flags().StringVarP(&output, "output", "o", "short", "Output width: short, wide")

	return cmd
}

// LogsOptions contains log viewing options
type LogsOptions struct {
	Env     string
	Action  string
	Level   string
	Limit   int
	Since   string
	TraceID string
	Format  string
	Output  string
}

// runLogs executes the logs command
func runLogs(opts LogsOptions) error {
	// Open state database
	db, err := openStateDB()
	if err != nil {
		return fmt.Errorf("ESTATE001: failed to open state database: %w", err)
	}
	defer db.Close()

	// Initialize audit store
	auditStore := state.NewAuditStore(db)
	if err := auditStore.InitAuditTable(); err != nil {
		return fmt.Errorf("ESTATE002: failed to initialize audit table: %w", err)
	}

	// Parse since duration
	var sinceTime time.Time
	if opts.Since != "" {
		duration, err := parseDuration(opts.Since)
		if err != nil {
			return fmt.Errorf("invalid duration format: %w", err)
		}
		sinceTime = time.Now().Add(-duration)
	}

	// Build query options
	queryOpts := state.QueryOptions{
		Env:     opts.Env,
		Action:  opts.Action,
		Limit:   opts.Limit,
		Since:   sinceTime,
		TraceID: opts.TraceID,
	}

	if opts.Level != "" {
		queryOpts.Level = state.AuditLevel(opts.Level)
	}

	// Query audit logs
	ctx := context.Background()
	events, err := auditStore.Query(ctx, queryOpts)
	if err != nil {
		return fmt.Errorf("failed to query audit logs: %w", err)
	}

	// Display results
	if len(events) == 0 {
		if opts.Format == "json" {
			fmt.Println("[]")
		} else {
			fmt.Println("No audit logs found matching the criteria.")
		}
		return nil
	}

	if opts.Format == "json" {
		return printLogsJSON(events)
	}

	printLogs(events, opts.Output == "wide")
	return nil
}

// printLogsJSON outputs audit events as JSON for scripting
func printLogsJSON(events []state.AuditEvent) error {
	type jsonEvent struct {
		ID        string                 `json:"id"`
		TraceID   string                 `json:"trace_id,omitempty"`
		Timestamp string                 `json:"timestamp"`
		Level     string                 `json:"level"`
		Action    string                 `json:"action"`
		Step      string                 `json:"step,omitempty"`
		Status    string                 `json:"status,omitempty"`
		Env       string                 `json:"env,omitempty"`
		Duration  string                 `json:"duration,omitempty"`
		Message   string                 `json:"message"`
		Error     string                 `json:"error,omitempty"`
		ErrorCode string                 `json:"error_code,omitempty"`
		RequestID string                 `json:"request_id,omitempty"`
		Details   map[string]interface{} `json:"details,omitempty"`
	}

	out := make([]jsonEvent, 0, len(events))
	for _, e := range events {
		je := jsonEvent{
			ID:        e.ID,
			TraceID:   e.TraceID,
			Timestamp: e.Timestamp.Format(time.RFC3339),
			Level:     string(e.Level),
			Action:    e.Action,
			Step:      e.Step,
			Status:    e.Status,
			Env:       e.Env,
			Message:   e.Message,
			Error:     e.Error,
			ErrorCode: e.ErrorCode,
			RequestID: e.RequestID,
			Details:   e.Details,
		}
		if e.Duration > 0 {
			je.Duration = e.Duration.Round(time.Second).String()
		}
		out = append(out, je)
	}

	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// printLogs displays audit events in a formatted table
func printLogs(events []state.AuditEvent, wide bool) {
	fmt.Println()
	color.Cyan("Audit Logs (%d entries):", len(events))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tTRACE\tLEVEL\tACTION\tSTEP\tSTATUS\tENV\tDURATION\tMESSAGE")
	fmt.Fprintln(w, "----\t-----\t-----\t------\t----\t------\t---\t--------\t-------")

	// Determine format settings based on width mode
	timeFmt := "2006-01-02 15:04:05"
	traceLen := 16
	msgLen := 60
	if !wide {
		timeFmt = "2006-01-02 15:04"
		traceLen = 12
		msgLen = 40
	}

	for _, event := range events {
		// Format timestamp (with seconds in both modes now)
		timestamp := event.Timestamp.Format(timeFmt)

		// Format trace ID
		traceStr := "-"
		if event.TraceID != "" {
			if !wide && len(event.TraceID) > traceLen {
				traceStr = event.TraceID[:traceLen] + "..."
			} else {
				traceStr = event.TraceID
			}
		}

		// Color code level
		levelStr := string(event.Level)
		switch event.Level {
		case state.AuditLevelError:
			levelStr = color.RedString(levelStr)
		case state.AuditLevelWarning:
			levelStr = color.YellowString(levelStr)
		default:
			levelStr = color.GreenString(levelStr)
		}

		// Format status with color
		statusStr := formatStatus(event.Status)

		// Format step
		stepStr := event.Step
		if stepStr == "" {
			stepStr = "-"
		}

		// Format duration
		durationStr := ""
		if event.Duration > 0 {
			durationStr = event.Duration.Round(time.Second).String()
		}

		// Format env
		envStr := event.Env
		if envStr == "" {
			envStr = "-"
		}

		// Build message with error code if present
		msg := truncateString(event.Message, msgLen)
		if event.ErrorCode != "" {
			msg = event.ErrorCode + ": " + msg
		}

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
			timestamp,
			traceStr,
			levelStr,
			event.Action,
			stepStr,
			statusStr,
			envStr,
			durationStr,
			msg,
		)
	}

	w.Flush()

	// Show error details at the bottom for visibility
	hasErrors := false
	for _, event := range events {
		if event.Level == state.AuditLevelError && event.Error != "" {
			hasErrors = true
			color.Red("\n  Error in [%s/%s]:", event.Action, event.Step)
			if event.ErrorCode != "" {
				fmt.Printf("    Code:       %s\n", event.ErrorCode)
			}
			if event.RequestID != "" {
				fmt.Printf("    Request ID: %s\n", event.RequestID)
			}
			fmt.Printf("    Message:    %s\n", event.Error)
		}
	}

	// Summary footer with counts by level
	fmt.Println()
	infoCount, warnCount, errCount := 0, 0, 0
	for _, event := range events {
		switch event.Level {
		case state.AuditLevelInfo:
			infoCount++
		case state.AuditLevelWarning:
			warnCount++
		case state.AuditLevelError:
			errCount++
		}
	}

	summary := fmt.Sprintf("  %s %d info", color.GreenString("●"), infoCount)
	if warnCount > 0 {
		summary += fmt.Sprintf("  %s %d warn", color.YellowString("●"), warnCount)
	}
	if errCount > 0 {
		summary += fmt.Sprintf("  %s %d error", color.RedString("●"), errCount)
	}
	fmt.Println(summary)

	if hasErrors {
		fmt.Println()
		color.Yellow("  Hint: Use --trace <id> to see full pipeline for a specific run.")
		color.Yellow("  Hint: See docs/error-code-matrix.md for error code reference.")
	}

	fmt.Println()
}

// formatStatus returns a color-coded status string
func formatStatus(status string) string {
	switch status {
	case "ok":
		return color.GreenString("ok")
	case "fail":
		return color.RedString("fail")
	case "skip":
		return color.YellowString("skip")
	case "":
		return "-"
	default:
		return status
	}
}

// truncateString truncates a string to max length with ellipsis
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

// parseDuration parses a duration string (e.g., "1h", "24h", "7d")
func parseDuration(s string) (time.Duration, error) {
	// Handle days
	if len(s) > 0 && s[len(s)-1] == 'd' {
		days := 0
		if _, err := fmt.Sscanf(s, "%dd", &days); err == nil {
			return time.Duration(days) * 24 * time.Hour, nil
		}
	}

	// Use standard duration parsing
	return time.ParseDuration(s)
}

// defaultDBPath returns the state database path.
// It checks INFRACAST_STATE_DB env var first, then falls back to .infra/state.db.
func defaultDBPath() string {
	if p := os.Getenv("INFRACAST_STATE_DB"); p != "" {
		return p
	}
	return ".infra/state.db"
}

// openStateDB opens the state database
func openStateDB() (*sql.DB, error) {
	return sql.Open("sqlite3", defaultDBPath())
}
