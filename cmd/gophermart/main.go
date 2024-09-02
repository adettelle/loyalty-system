package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/adettelle/loyalty-system/internal/accrualservice"
	"github.com/adettelle/loyalty-system/internal/gophermart/api"
	"github.com/adettelle/loyalty-system/internal/gophermart/config"
	"github.com/adettelle/loyalty-system/internal/gophermart/database"
	"github.com/adettelle/loyalty-system/internal/gophermart/model"
	"github.com/adettelle/loyalty-system/internal/migrator"
)

func main() {
	conf, err := config.New()
	if err != nil {
		log.Fatal(err)
	}

	migrator.MustApplyMigrations(conf.DBUri)

	db, err := database.Connect(conf.DBUri)
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()

	gmStorage := model.NewGophermartStorage(db)

	storage := &api.GophermartHandlers{
		GmStorage: gmStorage,
		SecretKey: []byte(conf.Key),
	}

	address := conf.Address //"localhost:8080"
	fmt.Println("Starting server at address:", address)

	r := api.NewRouter(storage)

	client := &http.Client{
		Timeout: time.Second * 2, // интервал ожидания: 2 секунды
	}

	accrualSystem := accrualservice.NewAccrualSystem(gmStorage, conf.AccrualSystemAddress, client)

	accrualSystem.AccrualLoop(context.Background())

	err = http.ListenAndServe(address, r)
	if err != nil {
		log.Fatal(err)
	}
}
