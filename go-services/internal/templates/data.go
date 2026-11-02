// internal/templates/data.go
package templates

type BaseData struct {
	User    *User
	Flashes []string
}

type User struct {
	ID       int64
	Username string
	Email    string
	// MustChangePassword is set for users whose password leaked in the breach.
	MustChangePassword bool
}
