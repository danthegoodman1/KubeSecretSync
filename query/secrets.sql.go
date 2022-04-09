// Code generated by sqlc. DO NOT EDIT.
// source: secrets.sql

package query

import (
	"context"

	"github.com/jackc/pgtype"
)

const getSecret = `-- name: GetSecret :one
SELECT ns, secret_name, manifest, created_at, updated_at, manifest_hash
FROM kss_secrets
WHERE ns = $1
AND secret_name = $2
`

type GetSecretParams struct {
	Ns         string
	SecretName string
}

func (q *Queries) GetSecret(ctx context.Context, arg GetSecretParams) (KssSecret, error) {
	row := q.db.QueryRow(ctx, getSecret, arg.Ns, arg.SecretName)
	var i KssSecret
	err := row.Scan(
		&i.Ns,
		&i.SecretName,
		&i.Manifest,
		&i.CreatedAt,
		&i.UpdatedAt,
		&i.ManifestHash,
	)
	return i, err
}

const upsertSecret = `-- name: UpsertSecret :execrows
INSERT INTO kss_secrets (ns, secret_name, manifest, manifest_hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (ns, secret_name)
DO UPDATE
SET manifest = EXCLUDED.manifest,
updated_at = NOW(),
manifest_hash = EXCLUDED.manifest_hash
WHERE kss_secrets.manifest_hash != EXCLUDED.manifest_hash
`

type UpsertSecretParams struct {
	Ns           string
	SecretName   string
	Manifest     pgtype.JSON
	ManifestHash string
}

func (q *Queries) UpsertSecret(ctx context.Context, arg UpsertSecretParams) (int64, error) {
	result, err := q.db.Exec(ctx, upsertSecret,
		arg.Ns,
		arg.SecretName,
		arg.Manifest,
		arg.ManifestHash,
	)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected(), nil
}