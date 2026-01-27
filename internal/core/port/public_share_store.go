package port

import (
	"context"

	"github.com/bornholm/corpus/internal/core/model"
)

type PublicShareStore interface {
	// FindPublicShareByToken retrieves a public share by its token, or returns port.ErrNotFound
	FindPublicShareByToken(ctx context.Context, token string) (model.PersistedPublicShare, error)

	// SavePublicShare saves a public share
	SavePublicShare(ctx context.Context, publicShare model.OwnedPublicShare) (model.PersistedPublicShare, error)
	// DeletePublicShare deletes a public share by its id
	DeletePublicShare(ctx context.Context, id model.PublicShareID) error

	// GetPublicShareByID returns a persisted public share by its id
	GetPublicShareByID(ctx context.Context, id model.PublicShareID) (model.PersistedPublicShare, error)

	// QueryPublicShares queries the existing public shares given the query options
	QueryPublicShares(ctx context.Context, opts QueryPublicSharesOptions) ([]model.PersistedPublicShare, error)
}

type QueryPublicSharesOptions struct {
	Page  *int
	Limit *int

	HeaderOnly bool
}
