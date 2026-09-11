package models

import (
	"testing"

	"golang.org/x/crypto/bcrypt"
)

func TestUserString(t *testing.T) {
	u := User{Login: "alice"}
	if s := u.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestUsersString(t *testing.T) {
	us := Users{{Login: "a"}, {Login: "b"}}
	if s := us.String(); s == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestMaintainerFlagIsDistinctFromAdmin(t *testing.T) {
	if (User{Admin: false, Maintainer: true}).Admin {
		t.Fatal("maintainer flag must not implicitly change Admin in the model")
	}
	if !(User{Admin: true, Maintainer: true}).Maintainer {
		t.Fatal("maintainer flag should be representable on admin users")
	}
}
func TestUserSetPasswordHash(t *testing.T) {
	u := User{Password: "s3cret"}
	err := u.SetPasswordHash()
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}
	if u.PasswordHash == "" {
		t.Fatal("Expected non-empty password hash")
	}
	// Hash must differ from the plaintext password.
	if u.PasswordHash == "s3cret" {
		t.Error("Password hash should not equal the plaintext password")
	}
	// Hash must be verifiable with bcrypt.
	if err := bcrypt.CompareHashAndPassword([]byte(u.PasswordHash), []byte("s3cret")); err != nil {
		t.Errorf("bcrypt verification failed: %v", err)
	}
}

func TestUserSetPasswordHashEmpty(t *testing.T) {
	u := User{}
	err := u.SetPasswordHash()
	if err != nil {
		t.Fatalf("Unexpected error for empty password: %v", err)
	}
	// bcrypt still produces a hash for an empty string.
	if u.PasswordHash == "" {
		t.Error("Expected non-empty password hash even for empty password")
	}
}
