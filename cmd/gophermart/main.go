package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/adettelle/loyalty-system/internal/gophermart/api"
	"github.com/adettelle/loyalty-system/internal/gophermart/config"
	"github.com/adettelle/loyalty-system/internal/gophermart/database"
)

func main() {
	var uri string

	config, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	if config.DBUri != "" {
		uri = config.DBUri
	}

	db, err := database.Connect(uri)
	if err != nil {
		log.Fatal(err)
	}

	err = database.CreateTable(db, context.Background())
	if err != nil {
		log.Fatal(err)
	}

	defer db.Close()

	storage := &api.DBStorage{
		DB:        db,
		Ctx:       context.Background(),
		SecretKey: []byte(config.Key),
	}

	address := config.Address //"localhost:8080"
	fmt.Println("Starting server at address:", address)

	r := api.NewRouter(storage)

	err = http.ListenAndServe(address, r)
	if err != nil {
		log.Fatal(err)
	}
}
