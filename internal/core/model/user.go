package model

import (
	"github.com/rs/xid"
)

type UserID string

func NewUserID() UserID {
	return UserID(xid.New().String())
}

type User interface {
	WithID[UserID]

	Email() string

	Subject() string
	Provider() string

	DisplayName() string

	Roles() []string
}

type BaseUser struct {
	id          UserID
	displayName string
	email       string
	subject     string
	provider    string
	roles       []string
}

// Email implements [User].
func (u *BaseUser) Email() string {
	return u.email
}

// ID implements [User].
func (u *BaseUser) ID() UserID {
	return u.id
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

func CopyUser(user User) *BaseUser {
	return &BaseUser{
		id:          user.ID(),
		displayName: user.DisplayName(),
		email:       user.Email(),
		subject:     user.Subject(),
		provider:    user.Provider(),
		roles:       append([]string{}, user.Roles()...),
	}
}

func NewUser(provider, subject, email string, displayName string, roles ...string) *BaseUser {
	return &BaseUser{
		id:          NewUserID(),
		displayName: displayName,
		email:       email,
		subject:     subject,
		provider:    provider,
		roles:       roles,
	}
}

func (u *BaseUser) SetDisplayName(displayName string) {
	u.displayName = displayName
}

func (u *BaseUser) SetEmail(email string) {
	u.email = email
}

func (u *BaseUser) SetRoles(roles ...string) {
	u.roles = roles
}

type AuthTokenID string

func NewAuthTokenID() AuthTokenID {
	return AuthTokenID(xid.New().String())
}

type AuthToken interface {
	WithID[AuthTokenID]

	UserID() UserID
	Label() string
	Value() string
}

type BaseAuthToken struct {
	id     AuthTokenID
	userID UserID
	label  string
	value  string
}

// ID implements AuthToken.
func (t *BaseAuthToken) ID() AuthTokenID {
	return t.id
}

// UserID implements AuthToken.
func (t *BaseAuthToken) UserID() UserID {
	return t.userID
}

// Label implements AuthToken.
func (t *BaseAuthToken) Label() string {
	return t.label
}

// Value implements AuthToken.
func (t *BaseAuthToken) Value() string {
	return t.value
}

var _ AuthToken = &BaseAuthToken{}

func NewAuthToken(userID UserID, label, value string) *BaseAuthToken {
	return &BaseAuthToken{
		id:     NewAuthTokenID(),
		userID: userID,
		label:  label,
		value:  value,
	}
}
