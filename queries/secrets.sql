-- name: UpsertSecret :execrows
INSERT INTO kss_secrets (ns, secret_name, manifest, manifest_hash)
VALUES ($1, $2, $3, $4)
ON CONFLICT (ns, secret_name)
DO UPDATE
SET manifest = EXCLUDED.manifest,
updated_at = NOW(),
manifest_hash = EXCLUDED.manifest_hash
WHERE kss_secrets.manifest_hash != EXCLUDED.manifest_hash;

-- name: GetSecret :one
SELECT *
FROM kss_secrets
WHERE ns = $1
AND secret_name = $2;

-- name: ListAllSecrets :many
SELECT *
FROM kss_secrets;
