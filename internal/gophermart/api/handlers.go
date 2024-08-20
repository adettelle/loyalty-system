package api

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/adettelle/loyalty-system/internal/gophermart/model"
	"github.com/adettelle/loyalty-system/pkg/mware/security"
	"github.com/adettelle/loyalty-system/pkg/validation/luhn"
)

type DBStorage struct {
	Ctx       context.Context
	DB        *sql.DB
	SecretKey []byte // []byte("my_secret_key")
}

type Auth struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type WithdrawalTxResponse struct {
	OrderNumber string    `json:"order"`
	Points      float64   `json:"sum"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewWithdrawalTxResponse(transaction model.TransactionW) WithdrawalTxResponse {
	return WithdrawalTxResponse{
		OrderNumber: transaction.OrderNumber,
		Points:      transaction.Points,
		CreatedAt:   transaction.CreatedAt, // Формат даты — RFC3339
	}
}

type OrderResponse struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Points     *float64  `json:"points,omitempty"`
	Accrual    float64   `json:"accrual"`              //,omitempty
	Withdrawal float64   `json:"withdrawal,omitempty"` //
	CreatedAt  time.Time `json:"uploaded_at"`          // created_at
}

type Customer struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type PointsResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type WithdrawReq struct {
	OrderNumber string  `json:"order"`
	Sum         float64 `json:"sum"`
}

// type OrdersListResponse struct {
// 	Orders []OrderResponse `json:"orders"`
// }

func NewCustomer(customer model.Customer) Customer {
	return Customer{
		ID:    customer.ID,
		Login: customer.Login,
	}
}

func NewOrderResponse(order model.Order) OrderResponse {
	res := OrderResponse{
		Number:    order.Number,
		Status:    order.Status,
		CreatedAt: order.CreatedAt,
	}

	if order.Points > 0 {
		if *order.Transaction == model.TransactionAccrual {
			res.Accrual = order.Points
		} else if *order.Transaction == model.TransactionWithdrawal {
			res.Withdrawal = order.Points
		}
	}
	return res
}

func NewOrderListResponse(orders []model.Order) []OrderResponse {
	res := []OrderResponse{}
	for _, order := range orders {
		res = append(res, NewOrderResponse(order))
	}
	return res
}

func (s *DBStorage) Login(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	var auth Auth

	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &auth); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !security.VerifyUser(auth.Login, auth.Password, s.DB, s.Ctx) {
		w.WriteHeader(http.StatusUnauthorized) // неверная пара логин/пароль
		return
	}
	token, err := security.GenerateJwtToken(s.SecretKey, auth.Login)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)

	w.WriteHeader(http.StatusOK)
}

// Хендлер доступен только авторизованному пользователю
func (s *DBStorage) AddOrder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userLogin := r.Header.Get("x-user")

	var buf bytes.Buffer
	// var customer Customer

	// читаем тело запроса
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest) // неверный формат запроса
		return
	}

	numOrder := buf.String()
	if !luhn.CheckLuhn(numOrder) {
		w.WriteHeader(http.StatusUnprocessableEntity) // неверный формат номера заказа
		return
	}

	log.Println("checked by Luhn numOrder:", numOrder)

	orderExists, err := model.OrderExists(numOrder, s.DB, s.Ctx)
	if err != nil {
		log.Println("error in order exists:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !orderExists {
		err = model.AddNewOrder(userLogin, numOrder, s.DB, s.Ctx)
		if err != nil {
			log.Println("error in adding order:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted) // новый номер заказа принят в обработку
		return
	} else {
		// TODO GetUserByOrder должен возвращать целого юзера с логином!!!!!!!!!!!!!
		// тогда не нужен запрос GetLoginByID
		customerFromModel, err := model.GetUserByOrder(numOrder, s.DB, s.Ctx)
		customer := NewCustomer(customerFromModel)

		if err != nil {
			log.Println("error in get user by order:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		// loginToCheck, err := model.GetLoginByID(idUser, s.DB, s.Ctx)
		// if err != nil {
		// 	// правильно ли выбрала тип ошибки?????????
		// 	w.WriteHeader(http.StatusInternalServerError)
		// 	return
		// }
		if userLogin == customer.Login { //loginToCheck
			w.WriteHeader(http.StatusOK) // номер заказа уже был загружен этим пользователем
			return
		}
		w.WriteHeader(http.StatusConflict) // номер заказа уже был загружен другим пользователем
		return
	}
}

// Хендлер доступен только авторизованному пользователю
func (s *DBStorage) GetOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	userLogin := r.Header.Get("x-user")
	log.Println("userLogin:", userLogin)
	// в некоторых местах и такой проверки нет!!!!!!!!!!!!!!!!!!!!
	// TODO userLogin не моежт быть пустым, он проверяется в middleware. Убрать здесь и везде
	if userLogin == "" { // достаточно ли проверки, что это не пустая строка??????????????
		w.WriteHeader(http.StatusUnauthorized) // пользователь не авторизован
		return
	}
	customer, err := model.GetCustomerByLogin(userLogin, s.DB, s.Ctx)
	log.Println("customer:", *customer)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError) // ошибка с БД
		return
	}
	if customer == nil {
		log.Println("customer == nil")
		w.WriteHeader(http.StatusNotFound) // это значит, нет такого пользователя
		return
	}

	orders, err := model.GetOrdersByUser(customer.ID, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println("orders:", orders)
	if len(orders) == 0 {
		log.Println("len(orders) == 0")
		w.WriteHeader(http.StatusNoContent) // нет данных для ответа
		return
	}
	orderListResponse := NewOrderListResponse(orders)
	log.Println(orderListResponse)
	resp, err := json.Marshal(NewOrderListResponse(orders))
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(resp)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// Хендлер доступен только авторизованному пользователю
func (s *DBStorage) GetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	userLogin := r.Header.Get("x-user")

	customer, err := model.GetCustomerByLogin(userLogin, s.DB, s.Ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError) // ошибка с БД
		return
	}
	if customer == nil {
		w.WriteHeader(http.StatusNotFound) // нет такого пользователя
		return
	}
	pointsAccrual, err := model.GetAccrualPoints(customer.ID, s.DB, s.Ctx)
	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pointsWithdrawal, err := model.GetWithdrawalPoints(customer.ID, s.DB, s.Ctx)
	if err != nil && err != sql.ErrNoRows {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	points := PointsResponse{
		Current:   pointsAccrual - pointsWithdrawal,
		Withdrawn: pointsWithdrawal,
	}

	resp, err := json.Marshal(points)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(resp)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Хендлер доступен только авторизованному пользователю
func (s *DBStorage) PostWithdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userLogin := r.Header.Get("x-user")
	if userLogin == "" {
		w.WriteHeader(http.StatusUnauthorized) // пользователь не авторизован
	}
	customer, err := model.GetCustomerByLogin(userLogin, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError) // ошибка с БД
		return
	}
	if customer == nil {
		w.WriteHeader(http.StatusNotFound) // это значит, нет такого пользователя
		return
	}

	var buf bytes.Buffer
	var wreq WithdrawReq

	// читаем тело запроса
	_, err = buf.ReadFrom(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &wreq); err != nil {
		log.Println("error in unmarshalling:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if !luhn.CheckLuhn(wreq.OrderNumber) {
		w.WriteHeader(http.StatusUnprocessableEntity) // неверный номер заказа
		return
	}

	sumInAccount, err := model.GetAccrualPoints(customer.ID, s.DB, s.Ctx)
	if err != nil {
		log.Printf("error %v in getting accrual points by user id %d", err, customer.ID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println("sumInAccount:", sumInAccount)
	log.Println("wreq.Sum:", wreq.Sum)

	if sumInAccount < wreq.Sum {
		w.WriteHeader(http.StatusPaymentRequired) // на счету недостаточно средств
		return
	}

	err = model.Withdraw(wreq.OrderNumber, wreq.Sum, customer.ID, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError) // внутренняя ошибка сервера
		return
	}
	w.WriteHeader(http.StatusOK)
}

func NewTransactionWListResponse(transactions []model.TransactionW) []WithdrawalTxResponse {
	res := []WithdrawalTxResponse{}
	for _, transaction := range transactions {
		res = append(res, NewWithdrawalTxResponse(transaction))
	}
	return res
}

// Хендлер доступен только авторизованному пользователю
func (s *DBStorage) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	userLogin := r.Header.Get("x-user")
	if userLogin == "" {
		w.WriteHeader(http.StatusUnauthorized) // пользователь не авторизован
	}
	customer, err := model.GetCustomerByLogin(userLogin, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError) // ошибка с БД
		return
	}
	if customer == nil {
		w.WriteHeader(http.StatusNotFound) // это значит, нет такого пользователя
		return
	}

	transactions, err := model.WithdrawalsByUser(customer.ID, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(transactions) == 0 {
		w.WriteHeader(http.StatusNoContent) // 204 — нет ни одного списания
		return
	}

	resp, err := json.Marshal(NewTransactionWListResponse(transactions)) // NewTransactionWListResponse(transactions)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(resp)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (s *DBStorage) RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	log.Println("In RegisterCustomer handler")
	var buf bytes.Buffer
	var customer Customer

	// читаем тело запроса
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest) // неверный формат запроса
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &customer); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	err = model.AddUser(customer.Login, customer.Password, s.DB, s.Ctx)
	if err != nil {
		if model.IsUserExistsErr(err) {
			w.WriteHeader(http.StatusConflict)
			return
		}
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token, err := security.GenerateJwtToken(s.SecretKey, customer.Login)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// автоматическая аутентификация пользователя после успешной регистрации
	w.Header().Set("Authorization", "Bearer "+token)

	w.WriteHeader(http.StatusOK)
}
