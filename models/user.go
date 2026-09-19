package models

import (
	"encoding/json"
	"strings"
	"time"

	"github.com/gobuffalo/nulls"
	"github.com/gobuffalo/pop/v6"
	"github.com/gobuffalo/validate/v3"
	"github.com/gobuffalo/validate/v3/validators"
	"github.com/gofrs/uuid"
	"github.com/pkg/errors"
	"golang.org/x/crypto/bcrypt"
)

// User is a generated model from buffalo-auth, it serves as the base for username/password authentication.
type User struct {
	ID           uuid.UUID `json:"id" db:"id"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	UpdatedAt    time.Time `json:"updated_at" db:"updated_at"`
	Login        string    `json:"login" db:"login"`
	Admin        bool      `json:"-" db:"admin"`
	Maintainer   bool      `json:"-" db:"maintainer"`
	Approved     bool      `json:"-" db:"approved"`
	Shared       bool      `json:"-" db:"shared"`
	PasswordHash string    `json:"-" db:"password_hash"`

	// Account role for restricted profiles (issue #107). Empty = regular
	// user (full read/write for non-admins, as before).
	Role string `json:"role" db:"role"`

	// Volunteer management columns (issue #107). Contact fields are
	// editable by the user and admins; flags + remark are admin-only.
	FirstName  string `json:"first_name" db:"first_name"`
	LastName   string `json:"last_name" db:"last_name"`
	Address    string `json:"address" db:"address"`
	PostalCode string `json:"postal_code" db:"postal_code"`
	City       string `json:"city" db:"city"`
	Email      string `json:"email" db:"email"`
	Phone      string `json:"phone" db:"phone"`

	FosterFamily  bool `json:"foster_family" db:"foster_family"`
	Transporter   bool `json:"transporter" db:"transporter"`
	BoardMember   bool `json:"board_member" db:"board_member"`
	Committee     bool `json:"committee" db:"committee"`
	Coordinator   bool `json:"coordinator" db:"coordinator"`
	Veterinarian  bool `json:"veterinarian" db:"veterinarian"`
	Referent      bool `json:"referent" db:"referent"`
	TeamLeader    bool `json:"team_leader" db:"team_leader"`
	Caregiver     bool `json:"caregiver" db:"caregiver"`
	CareAssistant bool `json:"care_assistant" db:"care_assistant"`
	Helper        bool `json:"helper" db:"helper"`

	Remark nulls.String `json:"remark" db:"remark"`

	Password             string `json:"-" db:"-"`
	PasswordConfirmation string `json:"-" db:"-"`
}

// Account roles (issue #107).
const (
	UserRoleRegular   = ""             // default: full user rights
	UserRoleReader    = "lecteur"      // view-only everywhere, own account
	UserRoleScientist = "scientifique" // view all animal tabs (don hidden), reports, animals; vet visits only
	UserRoleSPW       = "spw"          // view general/discovery/intake/outtake tabs, reports, animals, users; own account only
)

// Role display names for the admin UI.
var UserRoleNames = map[string]string{
	UserRoleRegular:   "Utilisateur",
	UserRoleReader:    "Lecteur",
	UserRoleScientist: "Scientifique (U Liège, DEMNA)",
	UserRoleSPW:       "SPW",
}

// IsReader reports whether the account is the read-only "Lecteur" role.
func (u *User) IsReader() bool { return u.Role == UserRoleReader }

// IsScientist reports whether the account is the "Scientifique" role
// (U Liège / DEMNA).
func (u *User) IsScientist() bool { return u.Role == UserRoleScientist }

// IsSPW reports whether the account is the restricted "SPW" role.
func (u *User) IsSPW() bool { return u.Role == UserRoleSPW }

// IsRestricted reports whether the account has a limited role (anything but
// a regular user/admin).
func (u *User) IsRestricted() bool { return u.Role != UserRoleRegular }

// DisplayName returns the volunteer name when known, the login otherwise.
func (u *User) DisplayName() string {
	name := strings.TrimSpace(u.FirstName + " " + u.LastName)
	if name == "" {
		return u.Login
	}
	return name
}

// SetPasswordHash update password hash based on password
func (u *User) SetPasswordHash() error {
	ph, err := bcrypt.GenerateFromPassword([]byte(u.Password), bcrypt.DefaultCost)
	if err != nil {
		return errors.WithStack(err)
	}
	u.PasswordHash = string(ph)
	return nil
}

// Create wraps up the pattern of encrypting the password and
// running validations. Useful when writing tests.
func (u *User) Create(tx *pop.Connection) (*validate.Errors, error) {
	u.Login = strings.ToLower(u.Login)
	err := u.SetPasswordHash()
	if err != nil {
		return validate.NewErrors(), errors.WithStack(err)
	}
	return tx.ValidateAndCreate(u)
}

// String is not required by pop and may be deleted
func (u User) String() string {
	ju, _ := json.Marshal(u)
	return string(ju)
}

// Users is not required by pop and may be deleted
type Users []User

// String is not required by pop and may be deleted
func (u Users) String() string {
	ju, _ := json.Marshal(u)
	return string(ju)
}

// Validate gets run every time you call a "pop.Validate*" (pop.ValidateAndSave, pop.ValidateAndCreate, pop.ValidateAndUpdate) method.
// This method is not required and may be deleted.
func (u *User) Validate(tx *pop.Connection) (*validate.Errors, error) {
	var err error
	return validate.Validate(
		&validators.StringIsPresent{Field: u.Login, Name: "Login"},
		&validators.StringIsPresent{Field: u.PasswordHash, Name: "PasswordHash"},
		// check to see if the login address is already taken:
		&validators.FuncValidator{
			Field:   u.Login,
			Name:    "Login",
			Message: "%s is already taken",
			Fn: func() bool {
				var b bool
				q := tx.Where("login = ?", u.Login)
				if u.ID != uuid.Nil {
					q = q.Where("id != ?", u.ID)
				}
				b, err = q.Exists(u)
				if err != nil {
					return false
				}
				return !b
			},
		},
	), err
}

// ValidateCreate gets run every time you call "pop.ValidateAndCreate" method.
// This method is not required and may be deleted.
func (u *User) ValidateCreate(tx *pop.Connection) (*validate.Errors, error) {
	var err error
	return validate.Validate(
		&validators.StringIsPresent{Field: u.Password, Name: "Password"},
		&validators.StringsMatch{Name: "Password", Field: u.Password, Field2: u.PasswordConfirmation, Message: "Password does not match confirmation"},
	), err
}

// ValidateUpdate gets run every time you call "pop.ValidateAndUpdate" method.
func (u *User) ValidateUpdate(tx *pop.Connection) (*validate.Errors, error) {
	var err error
	return validate.Validate(
		&validators.StringsMatch{Name: "Password", Field: u.Password, Field2: u.PasswordConfirmation, Message: "Password does not match confirmation"},
	), err
}
