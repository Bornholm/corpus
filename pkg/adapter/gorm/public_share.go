package gorm

import "time"

type PublicShare struct {
	ID string `gorm:"primaryKey;autoIncrement:false"`

	CreatedAt time.Time
	UpdatedAt time.Time

	Title       string
	Description string

	Token string `gorm:"unique"`

	Owner   *User
	OwnerID string

	Collections []*Collection `gorm:"many2many:public_shares_collections;"`
}
