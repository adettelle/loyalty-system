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

	accrualSystem := accrualservice.NewAccrualSystem(db, config.AccrualSystemAddress)

	accrualSystem.AccrualLoop()

	// go func() {
	// 	ticker := time.NewTicker(time.Second * 2)

	// 	for range ticker.C {
	// 		ordersWithNewStatus, err := model.GetAllNewOrders(db, context.Background())
	// 		log.Println("Orders with new status:", ordersWithNewStatus)
	// 		if err != nil {
	// 			log.Println("err1:", err) // ??????????????????????
	// 			continue
	// 		}
	// 		for _, ord := range ordersWithNewStatus {
	// 			orderFromAccrual, err := accrualservice.GetOrderFromAccrualSystem(ord.Number, config.AccrualSystemAddress)
	// 			if err != nil {
	// 				log.Println("err2:", err) // ????????????????????
	// 				continue
	// 			}
	// 			log.Println("Orders from accrual system:", orderFromAccrual)

	// 			err = model.UpdateOrderStatus(orderFromAccrual.Status, ord.Number, db, context.Background())
	// 			if err != nil {
	// 				log.Println("err3:", err) // ????????????????????
	// 				continue
	// 			}

	// 			err = model.UpdateAccrualPoints(orderFromAccrual.Accrual, ord.Number, db, context.Background())
	// 			if err != nil {
	// 				log.Println("err4:", err) // ????????????????????
	// 				continue
	// 			}
	// 		}
	// 	}
	// }()

	err = http.ListenAndServe(address, r)
	if err != nil {
		log.Fatal(err)
	}
}
