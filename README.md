# KubeSecretSync

Syncing kubernetes secrets across clusters scalably and securely.

### Database and SQL

CockroachDB and Postgres are the supported databases. The database used should be accessibly by all KubeSecretSync pods in all clusters.

CockroachDB is the preferred database for KubeSecretSync, however in order to offer compatibility CRDB specific features have been omitted to maintain Postgres compatibility.

## Choosing Secrets to Sync

All secrets to sync must have the label `kube-secret-sync=true`.

## Configuration

Env vars are used to configure.

### `DSN`

This is the DSN of the database, either CockroachDB or Postgres.

### `ENCRYPTION_KEY`

The key used for encryption and decryption of secrets to and from the DB. Must be the same for leaders and followers.

### `LEADER`

If set to `1`, then this indicates that the node is the leader and should act as a writer to the DB.

If not set to `1` then it will follow the state of the DB, adding annotations to the secrets based on the last update time from the DB.

### `TICK_SECONDS`

The timeout and interval for the loop. Default `20`. Ensure to raise this if you have lots of secrets, as you want lots of time to ensure secrets are synced before restarting the loop.

### `DEBUG`

If set to `1`, will print debug logs.

### `PRETTY`

If set to `1`, will format with standard logrus format.

## Usage With Cert-manager to Sync Certs Across Clusters

### Labeling Cert Secrets

Following https://cert-manager.io/docs/usage/certificate/#creating-certificate-resources we can use the `spec.secretTemplate.labels` to add `kube-secret-sync=true` so that the secret will have the correct label when created.
