package migrator

import (
	"database/sql"
	"embed"
	"errors"
	"log"

	"github.com/golang-migrate/migrate/v4"
	"github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

const migrationsDir = "migration"

//go:embed migration/*.sql
var MigrationsFS embed.FS

func MustApplyMigrations(dbParams string) {
	srcDriver, err := iofs.New(MigrationsFS, migrationsDir)
	if err != nil {
		log.Fatal(err)
	}

	db, err := sql.Open("pgx", dbParams)
	if err != nil {
		log.Fatal(err)
	}

	driver, err := postgres.WithInstance(db, &postgres.Config{})
	if err != nil {
		log.Fatalf("unable to create db instance: %v", err)
	}

	migrator, err := migrate.NewWithInstance("migration_embeded_sql_files", srcDriver, "psql_db", driver)
	if err != nil {
		log.Fatalf("unable to create migration: %v", err)
	}

	defer func() {
		migrator.Close()
	}()

	if err = migrator.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		log.Fatalf("unable to apply migrations %v", err)
	}

	log.Printf("Migrations applied")
}
