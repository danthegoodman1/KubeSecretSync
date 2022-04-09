CREATE TABLE IF NOT EXISTS kss_secrets (
  ns TEXT NOT NULL,
  secret_name TEXT NOT NULL,
  manifest JSON NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
  manifest_hash TEXT NOT NULL,
  PRIMARY KEY(ns, secret_name)
);
