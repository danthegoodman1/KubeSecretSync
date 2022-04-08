-- Name: UpsertSecret :execrows
INSERT INTO kss_secrets (ns, secret_name, manifest)
VALUES ($1, $2, $3)
ON CONFLICT (ns, secret_name)
DO UPDATE
SET manifest = EXCLUDED.email,
updated_at = NOW();
