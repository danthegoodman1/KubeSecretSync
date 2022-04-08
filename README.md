# KubeSecretSync

Syncing kubernetes secrets across clusters scalably and securely.

### Database and SQL

CockroachDB and Postgres are the supported databases. The database used should be accessibly by all KubeSecretSync pods in all clusters.

CockroachDB is the preferred database for KubeSecretSync, however in order to offer compatibility CRDB specific features have been omitted to maintain Postgres compatibility.
