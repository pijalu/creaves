package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/gobuffalo/grift/grift"
)

func createAdmin(c *Context) error {
	u := &models.User{
		Login:    "admin",
		Password: "admin",
		Admin:    true,
		Approved: true,
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
