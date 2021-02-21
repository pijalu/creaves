package grifts

import (
	"creaves/models"
	"fmt"

	. "github.com/markbates/grift/grift"
)

var _ = Desc("db:create_admin", "Create admin account")
var _ = Add("db:create_admin", func(c *Context) error {
	u := &models.User{
		Email:    "admin",
		Password: "admin",
		Admin:    true,
		Approved: true,
	}

	if err := u.SetPasswordHash(); err != nil {
		return err
	}
	if exists, err := models.DB.Q().Where("Email = ?", u.Email).Exists(&models.User{}); err != nil {
		return err
	} else if exists {
		fmt.Printf("Admin already exists - skipping creation \n")
		return nil
	}
	fmt.Printf("Creating administrator acccount: %s/%s", u.Email, u.Password)
	return models.DB.Create(u)
})
