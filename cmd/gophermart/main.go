package main

import (
	"context"
	"fmt"
	"log"
	"net/http"

	"github.com/adettelle/loyalty-system/internal/accrualservice"
	"github.com/adettelle/loyalty-system/internal/gophermart/api"
	"github.com/adettelle/loyalty-system/internal/gophermart/config"
	"github.com/adettelle/loyalty-system/internal/gophermart/database"
	"github.com/adettelle/loyalty-system/internal/gophermart/model"
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

	// err = database.CreateTable(db, context.Background())
	// if err != nil {
	// 	log.Fatal(err)
	// }
	database.DoMigration(config.DBUri)

	db, err := database.Connect(uri)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	gmStorage := model.NewGophermartStorage(db, context.Background())

	storage := &api.GophermartHandlers{
		GmStorage: gmStorage,
		SecretKey: []byte(config.Key),
	}

	address := config.Address //"localhost:8080"
	fmt.Println("Starting server at address:", address)

	r := api.NewRouter(storage)

	accrualSystem := accrualservice.NewAccrualSystem(gmStorage, config.AccrualSystemAddress)

	accrualSystem.AccrualLoop()

	err = http.ListenAndServe(address, r)
	if err != nil {
		log.Fatal(err)
	}
}
