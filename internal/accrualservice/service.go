package accrualservice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/adettelle/loyalty-system/internal/gophermart/model"
)

// (количество задач, которое одновременно происходит в worker pool)
const workerLimit = 5

type AccrualSystem struct {
	// DB  *sql.DB
	URI       string
	GmStorage *model.GophermartStorage
	client    *http.Client
}

type OrderStatsResp struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

func NewAccrualSystem(gmStorage *model.GophermartStorage, uri string) *AccrualSystem {
	return &AccrualSystem{
		URI:       uri,
		GmStorage: gmStorage,
	}
}

// GET /api/orders/{number}
func GetOrderFromAccrualSystem(number string, url string, client *http.Client) (OrderStatsResp, error) {
	log.Printf("Retrieving order %s from accrual system", number)
	var ord OrderStatsResp

	url = url + "/api/orders/" + number
	log.Println("url:", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println("error in request:", err)
		return OrderStatsResp{}, err
	}

	resp, err := client.Do(req) //  http.DefaultClient.Do(req)
	if err != nil {
		log.Println("error in DefaultClient:", err)
		return OrderStatsResp{}, err
	}

	if resp.StatusCode != http.StatusOK {
		return OrderStatsResp{}, fmt.Errorf("error in getting order from accrual system: order %s, status %d", number, resp.StatusCode)
	}

	var buf bytes.Buffer

	_, err = buf.ReadFrom(resp.Body)
	if err != nil {
		log.Println("err in reading body:", err)
		return OrderStatsResp{}, err
	}

	if err := json.Unmarshal(buf.Bytes(), &ord); err != nil {
		log.Println("err in unmarshalling:", err)
		return OrderStatsResp{}, err
	}

	resp.Body.Close()

	resp.Header.Set("Content-Type", "application/json")

	return ord, nil
}

func (as *AccrualSystem) AccrualLoop() {
	jobs := make(chan model.Order, workerLimit)

	for range workerLimit {
		go as.worker(jobs)
	}

	go func() {
		ticker := time.NewTicker(time.Second * 2)

		for range ticker.C {
			ordersWithProcessingStatus, err := as.GmStorage.GetAllNewOrdersChangeToProcessing()
			log.Println("Orders with new status:", ordersWithProcessingStatus)
			if err != nil {
				log.Println("error in getting orders with status 'processing':", err)
				continue
			}
			for _, ord := range ordersWithProcessingStatus {
				jobs <- ord
				// orderFromAccrual, err := GetOrderFromAccrualSystem(ord.Number, as.URI)
				// if err != nil {
				// 	// возвращаем статус из processing в new
				// 	err = as.GmStorage.UpdateOrderStatus(model.StatusNew, ord.Number)
				// 	if err != nil {
				// 		log.Println("error in updating status (to new):", err)
				// 		continue
				// 	}
				// 	log.Println("error in getting orders from accrual system with changed status:", err)
				// 	continue
				// }
				// log.Println("Orders from accrual system:", orderFromAccrual)

				// err = as.GmStorage.UpdateOrderStatus(orderFromAccrual.Status, ord.Number)
				// if err != nil {
				// 	log.Println("error in updating status of orders:", err)
				// 	continue
				// }

				// err = as.GmStorage.UpdateAccrualPoints(orderFromAccrual.Accrual, ord.Number)
				// if err != nil {
				// 	log.Println("error in updating points of orders:", err)
				// 	continue
				// }
			}
		}
	}()
}

func (as *AccrualSystem) worker(jobs <-chan model.Order) {
	for order := range jobs {
		orderFromAccrual, err := GetOrderFromAccrualSystem(order.Number, as.URI, as.client)
		if err != nil {
			log.Println("error in getting orders from accrual system with changed status:", err)
			// возвращаем статус из processing в new
			err = as.GmStorage.UpdateOrderStatus(model.StatusNew, order.Number)
			if err != nil {
				log.Println("error in updating status (to new):", err)
				continue
			}
			continue
		}
		log.Println("Orders from accrual system:", orderFromAccrual)

		err = as.GmStorage.UpdateOrderStatus(orderFromAccrual.Status, order.Number)
		if err != nil {
			log.Println("error in updating status of orders:", err)
			continue
		}

		err = as.GmStorage.UpdateAccrualPoints(orderFromAccrual.Accrual, order.Number)
		if err != nil {
			log.Println("error in updating points of orders:", err)
			continue
		}
		//result <- order
	}
}
