package models

import (
	"testing"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gofrs/uuid"
)

// TestTodoValidate: description presence is required (issue #147).
func TestTodoValidate(t *testing.T) {
	tx := &pop.Connection{}
	empty := &Todo{Description: "   "}
	// StringIsPresent treats whitespace-only as missing
	verrs, err := empty.Validate(tx)
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if !verrs.HasAny() {
		t.Error("whitespace-only description must fail validation")
	}

	ok := &Todo{Description: "call the vet"}
	verrs, err = ok.Validate(tx)
	if err != nil {
		t.Fatalf("validate error: %v", err)
	}
	if verrs.HasAny() {
		t.Errorf("unexpected validation errors: %v", verrs.Errors)
	}
}

// TestTodoStatusClassification: dashboard color code — done or future →
// green; open within 8h of todo_date → yellow; open older → red (issue #147).
func TestTodoStatusClassification(t *testing.T) {
	now := time.Date(2026, 9, 19, 12, 0, 0, 0, time.UTC)

	cases := []struct {
		name string
		todo Todo
		want string
	}{
		{"done regardless of age", Todo{
			Description: "x", TodoDate: now.Add(-72 * time.Hour),
			DoneAt: nulls.NewTime(now.Add(-1 * time.Hour)),
		}, TodoStatusGreen},
		{"future open", Todo{
			Description: "x", TodoDate: now.Add(2 * time.Hour),
		}, TodoStatusGreen},
		{"due exactly now", Todo{
			Description: "x", TodoDate: now,
		}, TodoStatusYellow},
		{"7h59 overdue", Todo{
			Description: "x", TodoDate: now.Add(-7*time.Hour - 59*time.Minute),
		}, TodoStatusYellow},
		{"exactly 8h overdue", Todo{
			Description: "x", TodoDate: now.Add(-8 * time.Hour),
		}, TodoStatusYellow},
		{"8h01 overdue", Todo{
			Description: "x", TodoDate: now.Add(-8*time.Hour - time.Minute),
		}, TodoStatusRed},
		{"long overdue", Todo{
			Description: "x", TodoDate: now.Add(-72 * time.Hour),
		}, TodoStatusRed},
	}
	for _, tc := range cases {
		if got := tc.todo.Status(now); got != tc.want {
			t.Errorf("%s: Status = %q, want %q", tc.name, got, tc.want)
		}
	}
}

// TestTodoIsDone: IsDone keys off done_at only, not the assignee.
func TestTodoIsDone(t *testing.T) {
	open := Todo{Description: "x", DoneByID: nulls.NewUUID(uuid.Must(uuid.NewV4()))}
	if open.IsDone() {
		t.Error("todo with done_by but no done_at must not be done")
	}
	done := Todo{Description: "x", DoneAt: nulls.NewTime(time.Now())}
	if !done.IsDone() {
		t.Error("todo with done_at must be done")
	}
}

func TestTodoNextOccurrence(t *testing.T) {
	base := time.Date(2026, 9, 19, 10, 0, 0, 0, time.UTC)
	cases := []struct {
		recurrence string
		wantNil    bool
		want       time.Time
	}{
		{TodoRecurrenceNone, true, time.Time{}},
		{"bogus", true, time.Time{}},
		{TodoRecurrenceWeekly, false, base.AddDate(0, 0, 7)},
		{TodoRecurrenceMonthly, false, base.AddDate(0, 1, 0)},
		{TodoRecurrenceYearly, false, base.AddDate(1, 0, 0)},
	}
	for _, tc := range cases {
		todo := Todo{Description: "x", TodoDate: base, Recurrence: tc.recurrence}
		next := todo.NextOccurrence()
		if tc.wantNil {
			if next != nil {
				t.Errorf("recurrence %q: got %+v, want nil", tc.recurrence, next)
			}
			continue
		}
		if next == nil {
			t.Errorf("recurrence %q: got nil", tc.recurrence)
			continue
		}
		if !next.TodoDate.Equal(tc.want) {
			t.Errorf("recurrence %q: todo_date = %s, want %s", tc.recurrence, next.TodoDate, tc.want)
		}
		if next.Recurrence != tc.recurrence || next.Description != todo.Description {
			t.Errorf("recurrence %q: spawned todo must carry description+recurrence", tc.recurrence)
		}
		if next.IsDone() {
			t.Errorf("recurrence %q: spawned todo must be open", tc.recurrence)
		}
	}
}
