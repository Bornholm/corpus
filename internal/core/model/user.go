package model

type User interface {
	Username() string
	Provider() string
	Roles() []string
}

type ReadOnlyUser struct {
	username string
	provider string
	roles    []string
}

// Roles implements User.
func (u *ReadOnlyUser) Roles() []string {
	return u.roles
}

// Provider implements User.
func (u *ReadOnlyUser) Provider() string {
	return u.provider
}

// Username implements User.
func (u *ReadOnlyUser) Username() string {
	return u.username
}

func NewReadOnlyUser(username string, provider string, roles ...string) *ReadOnlyUser {
	return &ReadOnlyUser{
		username: username,
		provider: provider,
		roles:    roles,
	}
}

var _ User = &ReadOnlyUser{}

func UserString(u User) string {
	return u.Username() + "@" + u.Provider()
}
