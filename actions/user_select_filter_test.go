package actions

import (
	"testing"

	"creaves/models"
)

// TestFilterApprovedUsers: deactivated (unapproved) accounts must not be
// offered in person-picker selects (#205 item 8).
// Issue: https://github.com/pijalu/creaves/issues/205
func TestFilterApprovedUsers(t *testing.T) {
	approved := models.User{Login: "active", Approved: true}
	pending := models.User{Login: "pending", Approved: false}
	deactivated := models.User{Login: "deactivated", Approved: false}
	admin := models.User{Login: "admin", Admin: true, Approved: true}

	in := &models.Users{approved, pending, deactivated, admin}
	out := filterApprovedUsers(in)

	if len(*out) != 2 {
		t.Fatalf("filterApprovedUsers kept %d users, want 2", len(*out))
	}
	for _, u := range *out {
		if !u.Approved {
			t.Errorf("unapproved user %q leaked through filter", u.Login)
		}
	}

	// Empty and all-unapproved inputs must stay empty (never nil-deref).
	if got := filterApprovedUsers(&models.Users{}); len(*got) != 0 {
		t.Errorf("empty input produced %d users, want 0", len(*got))
	}
	if got := filterApprovedUsers(&models.Users{pending}); len(*got) != 0 {
		t.Errorf("all-unapproved input produced %d users, want 0", len(*got))
	}
}
