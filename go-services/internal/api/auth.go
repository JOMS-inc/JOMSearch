// internal/api/auth.go
package api

import (
	"net/http"

	"github.com/gorilla/sessions"
)

var store = sessions.NewCookieStore([]byte("your-secret-key-change-me"))

type User struct {
	ID    int64
	Email string
}

func currentUser(r *http.Request) *User {
	session, err := store.Get(r, "session")
	if err != nil {
		return nil
	}

	email, ok := session.Values["email"].(string)
	if !ok {
		return nil
	}

	return &User{Email: email} // you'd normally look up full user from db by ID
}
