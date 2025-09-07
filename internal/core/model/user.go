package model

type User interface {
	Subject() string
	DisplayName() string
	Provider() string
	Roles() []string
}

type BaseUser struct {
	displayName string
	subject     string
	provider    string
	roles       []string
}

// DisplayName implements User.
func (u *BaseUser) DisplayName() string {
	return u.displayName
}

// Provider implements User.
func (u *BaseUser) Provider() string {
	return u.provider
}

// Roles implements User.
func (u *BaseUser) Roles() []string {
	return u.roles
}

// Subject implements User.
func (u *BaseUser) Subject() string {
	return u.subject
}

var _ User = &BaseUser{}

func NewUser(provider, subject, displayName string, roles ...string) *BaseUser {
	return &BaseUser{
		displayName: displayName,
		subject:     subject,
		provider:    provider,
		roles:       roles,
	}
}
