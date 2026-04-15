package commands

import (
	"context"
	"database/sql"
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
  infracast logs --trace trc_1234567890`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLogs(LogsOptions{
				Env:     env,
				Action:  action,
				Level:   level,
				Limit:   limit,
				Since:   since,
				TraceID: traceID,
			})
		},
	}

	cmd.Flags().StringVarP(&env, "env", "e", "", "Filter by environment")
	cmd.Flags().StringVarP(&action, "action", "a", "", "Filter by action (deploy, provision, destroy, etc.)")
	cmd.Flags().StringVarP(&level, "level", "l", "", "Filter by level (INFO, WARN, ERROR)")
	cmd.Flags().IntVar(&limit, "limit", 20, "Maximum number of entries to show")
	cmd.Flags().StringVar(&since, "since", "", "Show logs since duration (e.g., 1h, 24h, 7d)")
	cmd.Flags().StringVar(&traceID, "trace", "", "Filter by trace ID (e.g., trc_1234567890)")

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
		fmt.Println("No audit logs found matching the criteria.")
		return nil
	}

	printLogs(events)
	return nil
}

// printLogs displays audit events in a formatted table
func printLogs(events []state.AuditEvent) {
	fmt.Println()
	color.Cyan("Audit Logs (%d entries):", len(events))
	fmt.Println()

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "TIME\tTRACE\tLEVEL\tACTION\tSTEP\tSTATUS\tENV\tDURATION\tMESSAGE")
	fmt.Fprintln(w, "----\t-----\t-----\t------\t----\t------\t---\t--------\t-------")

	for _, event := range events {
		// Format timestamp
		timestamp := event.Timestamp.Format("2006-01-02 15:04")

		// Format trace ID (short form)
		traceStr := "-"
		if event.TraceID != "" {
			if len(event.TraceID) > 12 {
				traceStr = event.TraceID[:12]
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
		statusStr := event.Status
		if statusStr == "" {
			statusStr = "-"
		} else if statusStr == "ok" {
			statusStr = color.GreenString("ok")
		} else if statusStr == "fail" {
			statusStr = color.RedString("fail")
		}

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
		msg := truncateString(event.Message, 40)
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
	for _, event := range events {
		if event.Level == state.AuditLevelError && event.Error != "" {
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

	w.Flush()
	fmt.Println()
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

// openStateDB opens the state database
func openStateDB() (*sql.DB, error) {
	// TODO: Get DB path from config
	dbPath := ".infra/state.db"
	return sql.Open("sqlite3", dbPath)
}
