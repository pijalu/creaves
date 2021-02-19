package grifts

import (
	"creaves/models"

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

	return models.DB.Create(u)
})
