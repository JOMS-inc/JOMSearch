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
}
