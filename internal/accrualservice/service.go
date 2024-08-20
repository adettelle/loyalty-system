package accrualservice

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/adettelle/loyalty-system/internal/gophermart/model"
)

type AccrualSystem struct {
	DB  *sql.DB
	URI string
}

type OrderStatsResp struct {
	Order   string  `json:"order"`
	Status  string  `json:"status"`
	Accrual float64 `json:"accrual,omitempty"`
}

func NewAccrualSystem(db *sql.DB, uri string) *AccrualSystem {
	return &AccrualSystem{
		DB:  db,
		URI: uri,
	}
}

// GET /api/orders/{number}
func GetOrderFromAccrualSystem(number string, url string) (OrderStatsResp, error) {
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

func (as *AccrualSystem) AccrualLoop() {
	go func() {
		ticker := time.NewTicker(time.Second * 2)

		for range ticker.C {
			ordersWithNewStatus, err := model.GetAllNewOrders(as.DB, context.Background())
			log.Println("Orders with new status:", ordersWithNewStatus)
			if err != nil {
				log.Println("err1:", err) // ??????????????????????
				continue
			}
			for _, ord := range ordersWithNewStatus {
				orderFromAccrual, err := GetOrderFromAccrualSystem(ord.Number, as.URI)
				if err != nil {
					log.Println("err2:", err) // ????????????????????
					continue
				}
				log.Println("Orders from accrual system:", orderFromAccrual)

				err = model.UpdateOrderStatus(orderFromAccrual.Status, ord.Number, as.DB, context.Background())
				if err != nil {
					log.Println("err3:", err) // ????????????????????
					continue
				}

				err = model.UpdateAccrualPoints(orderFromAccrual.Accrual, ord.Number, as.DB, context.Background())
				if err != nil {
					log.Println("err4:", err) // ????????????????????
					continue
				}
			}
		}
	}()
}
