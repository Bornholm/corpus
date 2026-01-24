package gorm

import (
	"time"
)

type User struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Subject  string `gorm:"index"`
	Provider string `gorm:"index"`

	DisplayName string
	Email       string `gorm:"unique"`

	AuthTokens  []*AuthToken  `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE;"`
	Documents   []*Document   `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE;"`
	Collections []*Collection `gorm:"foreignKey:OwnerID;constraint:OnDelete:CASCADE;"`

	Roles []*UserRole `gorm:"constraint:OnDelete:CASCADE;"`

	Active bool
}

type AuthToken struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Owner   *User
	OwnerID string

	Label string
	Value string `gorm:"unique"`
}

type UserRole struct {
	ID uint `gorm:"primaryKey"`

	CreatedAt time.Time

	User   *User
	UserID string `gorm:"index:user_role_index,unique"`

	Role string `gorm:"index:user_role_index,unique"`
}
