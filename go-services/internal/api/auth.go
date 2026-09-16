package api

import (
	"net/http"

	"github.com/gorilla/sessions"

	"github.com/JOMS-inc/JOMSearch/go-services/internal/templates"
)

var store = sessions.NewCookieStore([]byte("your-secret-key-change-me"))

func currentUser(r *http.Request) *templates.User {
	session, err := store.Get(r, "session")
	if err != nil {
		return nil
	}

	email, ok := session.Values["email"].(string)
	if !ok {
		return nil
	}

	return &templates.User{Email: email} // TODO: slå fuld bruger (inkl. Username) op i db
}