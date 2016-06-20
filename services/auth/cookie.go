package auth

import (
	"encoding/gob"
	"fmt"
	"github.com/gorilla/sessions"
	"net/http"
	"time"
)

var cookieStore *sessions.CookieStore
var cookieDuration int
var cookiePath string

type AuthCookie struct {
	Username		string
	Token			string
	ExpiredAt		time.Time
}

func InitCookieStore(cookie_keys [][]byte, cookie_path string) {
	cookieStore = sessions.NewCookieStore(cookie_keys...)
	gob.Register(&AuthCookie{})

	cookieDuration = 3600 * 24 * 7
	cookiePath = cookie_path
}

func NewAuthCookie(username string) *AuthCookie {
	return &AuthCookie {
		Username:	username,
		ExpiredAt:	time.Now().Add(time.Second * time.Duration(cookieDuration)),
	}
}

func CheckAuthCookie(r *http.Request) (*AuthCookie, error) {
	session, err := cookieStore.Get(r, "apparat")
	if err != nil {
		return nil, fmt.Errorf("could not read cookie: %v", err)
	}

	val := session.Values["auth"]
	var ac = &AuthCookie{}
	var ok bool
	if ac, ok = val.(*AuthCookie); !ok {
		return nil, fmt.Errorf("invalid cookie format, could not transform")
	}

	if time.Now().After(ac.ExpiredAt) {
		return nil, fmt.Errorf("expired cookie")
	}

	if len(ac.Username) == 0 {
		return nil, fmt.Errorf("invalid cookie format")
	}

	return ac, nil
}

func SetAuthCookie(r *http.Request, w http.ResponseWriter, ac *AuthCookie) error {
	session, err := cookieStore.Get(r, "apparat")
	if err != nil {
		return fmt.Errorf("could not read cookie: %v", err)
	}

	session.Options = &sessions.Options {
		Path:		cookiePath,
		MaxAge:		cookieDuration,
		HttpOnly:	true,
	}

	session.Values["auth"] = ac
	return session.Save(r, w)
}
