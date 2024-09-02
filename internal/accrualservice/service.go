package accrualservice

import (
	"bytes"
	"context"
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
	URI       string
	GmStorage *model.GophermartStorage
	client    *http.Client
}

type OrderStatsResp struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

func NewAccrualSystem(gmStorage *model.GophermartStorage, uri string, client *http.Client) *AccrualSystem {
	return &AccrualSystem{
		URI:       uri,
		GmStorage: gmStorage,
		client:    client,
	}
}

// GET /api/orders/{number}
func (as *AccrualSystem) GetOrderFromAccrualSystem(number string, url string) (OrderStatsResp, error) {
	log.Printf("Retrieving order %s from accrual system", number)
	var ord OrderStatsResp

	url = url + "/api/orders/" + number
	log.Println("url:", url)

	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		log.Println("error in request:", err)
		return OrderStatsResp{}, err
	}

	resp, err := as.client.Do(req)
	if err != nil {
		log.Println("error in client:", err)
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

func (as *AccrualSystem) AccrualLoop(ctx context.Context) {
	jobs := make(chan model.Order, workerLimit)

	for range workerLimit {
		go as.worker(ctx, jobs)
	}

	go func() {
		ticker := time.NewTicker(time.Second * 2)

		for range ticker.C {
			ordersWithProcessingStatus, err := as.GmStorage.GetAllNewOrdersChangeToProcessing(ctx)
			log.Println("Orders with new status:", ordersWithProcessingStatus)
			if err != nil {
				log.Println("error in getting orders with status 'processing':", err)
				continue
			}
			for _, ord := range ordersWithProcessingStatus {
				jobs <- ord
			}
		}
	}()
}

func (as *AccrualSystem) worker(ctx context.Context, jobs <-chan model.Order) {
	for order := range jobs {
		orderFromAccrual, err := as.GetOrderFromAccrualSystem(order.Number, as.URI)
		if err != nil {
			log.Println("error in getting orders from accrual system with changed status:", err)
			// возвращаем статус из processing в new
			err = as.GmStorage.UpdateOrderStatus(ctx, model.StatusNew, order.Number)
			if err != nil {
				log.Println("error in updating status (to new):", err)
				continue
			}
			continue
		}
		log.Println("Orders from accrual system:", orderFromAccrual)

		err = as.GmStorage.UpdateOrderStatus(ctx, orderFromAccrual.Status, order.Number)
		if err != nil {
			log.Println("error in updating status of orders:", err)
			continue
		}

		err = as.GmStorage.UpdateAccrualPoints(ctx, orderFromAccrual.Accrual, order.Number)
		if err != nil {
			log.Println("error in updating points of orders:", err)
			continue
		}
	}
}
