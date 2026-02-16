package test

import (
	"context"
	"net/http"
	"testing"
	"time"

	"github.com/rootlyhq/rootly-go"
)

func TestListShifts(t *testing.T) {
	client := SetupClient(t)

	now := time.Now()
	from := now.AddDate(0, 0, -30).Format(time.RFC3339)
	to := now.Format(time.RFC3339)

	ctx := context.Background()
	params := &rootly.ListShiftsParams{
		From: &from,
		To:   &to,
	}

	resp, err := client.ListShiftsWithResponse(ctx, params)
	if err != nil {
		t.Fatalf("Failed to call ListShifts: %v", err)
	}

	if resp.StatusCode() != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", resp.StatusCode())
	}

	if resp.ApplicationVndAPIJSON200 == nil {
		t.Fatal("Expected response body")
	}

	shiftList := resp.ApplicationVndAPIJSON200

	if len(shiftList.Data) == 0 {
		t.Skip("No shifts returned, skipping validation")
	}

	for _, shift := range shiftList.Data {
		if shift.ID == "" {
			t.Error("Shift ID should not be empty")
		}

		if shift.Type != "shifts" {
			t.Errorf("Expected type 'shifts', got '%s'", shift.Type)
		}

		attrs := shift.Attributes
		if attrs.ScheduleID == "" {
			t.Error("ScheduleId should not be empty")
		}

		if attrs.UserID.IsSpecified() {
			userId := attrs.UserID.MustGet()
			if userId <= 0 {
				t.Errorf("user_id should be positive, got %d", userId)
			}
		}

		if shift.Relationships != nil {
			if shift.Relationships.User != nil && shift.Relationships.User.Data.IsSpecified() {
				userData := shift.Relationships.User.Data.MustGet()
				if userData.Type != nil && *userData.Type != "users" {
					t.Errorf("Expected user relationship type 'users', got '%s'", *userData.Type)
				}
			}
		}
	}
}
