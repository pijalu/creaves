package grifts

import (
	"creaves/models"
	"fmt"

	grift "github.com/gobuffalo/grift/grift"
)

func createAdmin(c *grift.Context) error {
	u := &models.User{
		Login:      "admin",
		Password:   "admin",
		Admin:      true,
		Maintainer: true,
		Approved:   true,
	}

	if err := u.SetPasswordHash(); err != nil {
		return err
	}
	if exists, err := models.DB.Q().Where("Login = ?", u.Login).Exists(&models.User{}); err != nil {
		return err
	} else if exists {
		fmt.Printf("Admin already exists - skipping creation \n")
		return nil
	}
	fmt.Printf("Creating administrator acccount: %s/%s\n", u.Login, u.Password)
	return models.DB.Create(u)
}

// admin:reset-password — fixture/dev tool (BUG-50 E2E round): dump
// clones carry an `admin` row with an UNKNOWN password hash, so the
// skip-if-exists guard in createAdmin makes fixtures unloggable. This
// resets the existing row to admin/admin and force-enables admin +
// approved. NEVER run against a production database.
func init() {
	grift.Desc("admin:reset-password", "Reset the admin account to admin/admin (force admin+approved) — fixture/dev only")
	grift.Add("admin:reset-password", func(c *grift.Context) error {
		u := &models.User{}
		if err := models.DB.Where("login = ?", "admin").First(u); err != nil {
			// No admin row at all: fall back to plain creation.
			fmt.Println("no admin row — creating")
			return createAdmin(c)
		}
		u.Password = "admin"
		u.Admin = true
		u.Approved = true
		if err := u.SetPasswordHash(); err != nil {
			return err
		}
		if err := models.DB.Save(u); err != nil {
			return err
		}
		fmt.Printf("Admin password reset to admin/admin (admin=%v approved=%v)\n", u.Admin, u.Approved)
		return nil
	})
}
