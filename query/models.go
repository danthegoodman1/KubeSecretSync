// Code generated by sqlc. DO NOT EDIT.

package query

import (
	"time"

	"github.com/jackc/pgtype"
)

type KssSecret struct {
	Ns           string
	SecretName   string
	Manifest     pgtype.JSON
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ManifestHash string
}
