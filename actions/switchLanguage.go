package actions

import (
	"net/http"
	"time"

	"github.com/gobuffalo/buffalo"
	"github.com/pkg/errors"
)

func setLang(lang, url string, c buffalo.Context) error {
	// Set new current language using a cookie, for instance
	cookie := http.Cookie{
		Name:   "lang",
		Value:  lang,
		MaxAge: int((time.Hour * 24 * 265).Seconds()),
		Path:   "/",
	}
	http.SetCookie(c.Response(), &cookie)

	// Update language for the flash message
	T.Refresh(c, lang)

	c.Flash().Add("success", T.Translate(c, "users.language-changed", struct {
		Lang string
		Url  string
	}{
		Lang: lang,
		Url:  url,
	}))

	if url == "" {
		return c.Redirect(302, "/")
	}
	return c.Redirect(302, url)
}

func SwitchLanguage(c buffalo.Context) error {
	return setLang(c.Param("lang"), c.Param("url"), c)
}

func SwitchLanguagePost(c buffalo.Context) error {
	f := struct {
		Language string `form:"lang"`
		URL      string `form:"url"`
	}{}
	if err := c.Bind(&f); err != nil {
		return errors.WithStack(err)
	}
	return setLang(f.Language, f.URL, c)
}
