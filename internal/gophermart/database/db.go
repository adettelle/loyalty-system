package database

import (
	"database/sql"
	"embed"
	"log"

	"github.com/adettelle/loyalty-system/internal/migrator"
	_ "github.com/jackc/pgx/v5/stdlib"
)

const migrationsDir = "migration"

//go:embed migration/*.sql
var MigrationsFS embed.FS

func DoMigration(dburi string) { //db *sql.DB,
	db, err := Connect(dburi)
	if err != nil {
		log.Fatal(err)
	}
	// defer db.Close()
	// --- (1) ----
	// Восстанавливаем «Migrator»
	migrator := migrator.MustGetNewMigrator(MigrationsFS, migrationsDir)

	// --- (2) ----
	// Применяем миграции
	err = migrator.ApplyMigrations(db)
	if err != nil {
		log.Fatal(err)
	}

	log.Printf("Migrations applied!!")
}

func Connect(dbParams string) (*sql.DB, error) {
	log.Println("Connecting to DB", dbParams)
	db, err := sql.Open("pgx", dbParams)
	if err != nil {
		return nil, err
	}
	return db, nil
}
