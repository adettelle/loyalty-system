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

type AccrualSystem struct {
	// DB  *sql.DB
	URI       string
	GmStorage *model.GophermartStorage
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
func GetOrderFromAccrualSystem(number string, url string) (OrderStatsResp, error) {
	log.Printf("Retrieving order %s from accrual system", number)
	var ord OrderStatsResp

	url = url + "/api/orders/" + number
	log.Println("url:", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println("error in request:", err)
		return OrderStatsResp{}, err
	}

	resp, err := http.DefaultClient.Do(req)
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

// (количество задач, которое одновременно происходит в worker pool)
const workerLimit = 5

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
		orderFromAccrual, err := GetOrderFromAccrualSystem(order.Number, as.URI)
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

// где это должно быть???
// func MyMain() {
// 	// пусть не больше 5 запросов уходит
// 	jobs := make(chan model.Order, 10) // каакой размер канала????????????????
// 	// results := make(chan model.Order, 10) // каакой размер канала????????????????

// 	// как стартовать worker???????????????????
// 	// почему в прримере три отдельных цикла??? можно ли это будет один цикл? или два(w и j) (result отдельно)
// 	for range 5 {
// 		go worker(jobs) // он же не запустится??????????????????? , results
// 	}

// 	for _, ord := range ordersWithProcessingStatus {
// 		jobs <- ord
// 	}

// 	// for a := 1; a <= len(ordersWithProcessingStatus); a++ {
// 	// 	<-results
// 	// }
// }
