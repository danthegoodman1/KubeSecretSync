package migrations

import (
	"database/sql"
	"embed"

	migrate "github.com/rubenv/sql-migrate"
)

var (
	//go:embed *.sql
	Migrations embed.FS
)

func RunMigrations(db *sql.DB) (int, error) {
	src := migrate.EmbedFileSystemMigrationSource{
		FileSystem: Migrations,
		Root:       ".",
	}
	ms := migrate.MigrationSet{
		TableName: "migrations",
	}
	return ms.Exec(db, "postgres", src, migrate.Up)
}
