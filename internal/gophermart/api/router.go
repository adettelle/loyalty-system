package api

import (
	"github.com/adettelle/loyalty-system/pkg/mware"
	"github.com/go-chi/chi/v5"
)

func NewRouter(storage *DBStorage) chi.Router {
	r := chi.NewRouter()

	// конфигурирование сервера
	r.Post("/api/user/login", storage.Login)
	r.Post("/api/user/orders", mware.AuthMwr(storage.AddOrder))
	r.Get("/api/user/orders", mware.AuthMwr(storage.GetOrders))
	r.Get("/api/user/balance", mware.AuthMwr(storage.GetBalance))
	r.Post("/api/user/balance/withdraw", mware.AuthMwr(storage.PostWithdraw))
	r.Get("/api/user/withdrawals", mware.AuthMwr(storage.GetWithdrawals))
	return r
}
