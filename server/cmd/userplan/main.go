package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"server/internal/infra"
	"server/internal/sqlinline"
)

func main() {
	var (
		idFlag        string
		emailFlag     string
		planFlag      string
		quotaFlag     int
		keepUsageFlag bool
	)

	flag.StringVar(&idFlag, "id", "", "user ID to update (UUID)")
	flag.StringVar(&emailFlag, "email", "", "user email to update")
	flag.StringVar(&planFlag, "plan", "pro", "plan to assign (free, pro, supporter)")
	flag.IntVar(&quotaFlag, "quota", 50, "daily quota to enforce for the plan (set <=0 to keep current value)")
	flag.BoolVar(&keepUsageFlag, "keep-usage", false, "preserve current quota_used_today instead of resetting to 0")
	flag.Parse()

	userID := strings.TrimSpace(idFlag)
	email := strings.TrimSpace(emailFlag)
	plan := strings.TrimSpace(strings.ToLower(planFlag))

	if userID == "" && email == "" {
		exitWithError(errors.New("either -id or -email must be provided"))
	}
	if plan == "" {
		exitWithError(errors.New("-plan is required"))
	}
	switch plan {
	case "free", "pro", "supporter":
	default:
		exitWithError(fmt.Errorf("unsupported plan %q", plan))
	}

	dbURL := strings.TrimSpace(os.Getenv("DATABASE_URL"))
	if dbURL == "" {
		exitWithError(errors.New("DATABASE_URL is required"))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		exitWithError(fmt.Errorf("failed to connect database: %w", err))
	}
	defer pool.Close()

	logger := infra.NewLogger("cli").With().Str("cmd", "userplan").Logger()
	runner := infra.NewSQLRunner(pool, logger)

	lookupCtx, cancelLookup := context.WithTimeout(context.Background(), 5*time.Second)
	var rowData struct {
		ID    string
		Email string
		Plan  string
		Props []byte
	}
	var scanErr error
	if userID != "" {
		row := runner.QueryRow(lookupCtx, sqlinline.QSelectUserPlanByID, userID)
		scanErr = row.Scan(&rowData.ID, &rowData.Email, &rowData.Plan, &rowData.Props)
	} else {
		row := runner.QueryRow(lookupCtx, sqlinline.QSelectUserPlanByEmail, email)
		scanErr = row.Scan(&rowData.ID, &rowData.Email, &rowData.Plan, &rowData.Props)
	}
	cancelLookup()
	if scanErr != nil {
		exitWithError(fmt.Errorf("failed to load user: %w", scanErr))
	}

	props := map[string]any{}
	if len(rowData.Props) > 0 {
		if err := json.Unmarshal(rowData.Props, &props); err != nil {
			exitWithError(fmt.Errorf("failed to decode user properties: %w", err))
		}
	}

	if quotaFlag > 0 {
		props["quota_daily"] = quotaFlag
	}
	if !keepUsageFlag {
		props["quota_used_today"] = 0
	}
	props["quota_refreshed_at"] = time.Now().UTC().Format(time.RFC3339Nano)

	updatedProps, err := json.Marshal(props)
	if err != nil {
		exitWithError(fmt.Errorf("failed to encode user properties: %w", err))
	}

	updateCtx, cancelUpdate := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelUpdate()
	row := runner.QueryRow(updateCtx, sqlinline.QUpdateUserPlan, rowData.ID, plan, updatedProps)

	var (
		updatedID         string
		updatedEmail      string
		updatedPlan       string
		updatedPropsBytes []byte
	)
	if err := row.Scan(&updatedID, &updatedEmail, &updatedPlan, &updatedPropsBytes); err != nil {
		exitWithError(fmt.Errorf("failed to update user plan: %w", err))
	}

	resultProps := map[string]any{}
	if len(updatedPropsBytes) > 0 {
		_ = json.Unmarshal(updatedPropsBytes, &resultProps)
	}

	fmt.Printf("User %s (%s) updated to plan %s\n", updatedID, updatedEmail, updatedPlan)
	if quota, ok := resultProps["quota_daily"]; ok {
		fmt.Printf("quota_daily=%v\n", quota)
	}
	if used, ok := resultProps["quota_used_today"]; ok {
		fmt.Printf("quota_used_today=%v\n", used)
	}
	if refreshed, ok := resultProps["quota_refreshed_at"]; ok {
		fmt.Printf("quota_refreshed_at=%v\n", refreshed)
	}
}

func exitWithError(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
