package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

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
	log.Println(config)

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

	go func() {
		ticker := time.NewTicker(time.Second * 2)

		for range ticker.C {
			ordersWithNewStatus, err := model.GetAllNewOrders(db, context.Background())
			log.Println("Orders with new ststus:", ordersWithNewStatus)
			if err != nil {
				log.Println("err1:", err) // ??????????????????????
				continue
			}
			for _, ord := range ordersWithNewStatus {
				orderFromAccrual, err := GetOrderFromAccrualSystem(ord.Number, config.AccrualSystemAddress)
				if err != nil {
					log.Println("err2:", err) // ????????????????????
					continue
				}
				log.Println("Orders from accrual system:", orderFromAccrual)

				err = model.UpdateOrderStatus(orderFromAccrual.Status, ord.Number, db, context.Background())
				if err != nil {
					log.Println("err3:", err) // ????????????????????
					continue
				}

				err = model.UpdateAccrualPoints(orderFromAccrual.Accrual, ord.Number, db, context.Background())
				if err != nil {
					log.Println("err4:", err) // ????????????????????
					continue
				}
			}
		}
	}()

	err = http.ListenAndServe(address, r)
	if err != nil {
		log.Fatal(err)
	}
}

type OrderStatsResp struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

// GET /api/orders/{number}
func GetOrderFromAccrualSystem(number string, url string) (OrderStatsResp, error) {
	// log.Println("In GetStatusFromAccrualSystem")
	var ord OrderStatsResp

	// url :=  // "http://localhost:8081/api/orders/" + number
	url = url + "api/orders/" + number
	log.Println(url)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println("err1:", err)
		return OrderStatsResp{}, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		log.Println("err2:", err)
		return OrderStatsResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return OrderStatsResp{}, fmt.Errorf("error getting order from accrual system: order %s, ststus %d", number, resp.StatusCode)
	}

	var buf bytes.Buffer

	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		log.Println("err3:", err)
		return OrderStatsResp{}, err
	}

	if err := json.Unmarshal(buf.Bytes(), &ord); err != nil {
		log.Println("err4:", err)
		return OrderStatsResp{}, err
	}

	resp.Body.Close()

	log.Println(resp.StatusCode)
	log.Println("ord:", ord)

	resp.Header.Set("Content-Type", "application/json")

	return ord, nil
}
