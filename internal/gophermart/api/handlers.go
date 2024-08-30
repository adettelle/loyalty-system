package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"github.com/adettelle/loyalty-system/internal/gophermart/jwt"
	"github.com/adettelle/loyalty-system/internal/gophermart/model"

	// "github.com/adettelle/loyalty-system/pkg/mware/security"
	"github.com/adettelle/loyalty-system/pkg/validation/luhn"
)

type GophermartHandlers struct {
	// Ctx       context.Context
	// DB        *sql.DB
	GmStorage *model.GophermartStorage
	SecretKey []byte // []byte("my_secret_key")
}

type authRequestDTO struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type withdrawalTxResponseDTO struct {
	OrderNumber string    `json:"order"`
	Points      float64   `json:"sum"`
	CreatedAt   time.Time `json:"created_at"`
}

func NewWithdrawalTxResponse(transaction model.TxWithdraw) withdrawalTxResponseDTO {
	return withdrawalTxResponseDTO{
		OrderNumber: transaction.OrderNumber,
		Points:      transaction.Points,
		CreatedAt:   transaction.CreatedAt, // Формат даты — RFC3339
	}
}

func NewWithdrawalTxListResponse(transactions []model.TxWithdraw) []withdrawalTxResponseDTO {
	res := []withdrawalTxResponseDTO{}
	for _, transaction := range transactions {
		res = append(res, NewWithdrawalTxResponse(transaction))
	}
	return res
}

type orderResponseDTO struct {
	Number     string    `json:"number"`
	Status     string    `json:"status"`
	Points     *float64  `json:"points,omitempty"`
	Accrual    float64   `json:"accrual"`              //,omitempty
	Withdrawal float64   `json:"withdrawal,omitempty"` //
	CreatedAt  time.Time `json:"uploaded_at"`          // created_at
}

type customerDTO struct {
	ID       int    `json:"id"`
	Login    string `json:"login"`
	Password string `json:"password"`
}

type pointsResponseDTO struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

type withdrawRequest struct {
	OrderNumber string  `json:"order"`
	Sum         float64 `json:"sum"`
}

// type OrdersListResponse struct {
// 	Orders []OrderResponse `json:"orders"`
// }

func NewCustomer(customer *model.Customer) *customerDTO {
	return &customerDTO{
		ID:    customer.ID,
		Login: customer.Login,
	}
}

func NewOrderResponse(order model.Order) *orderResponseDTO {
	res := orderResponseDTO{
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
	return &res
}

func NewOrderListResponse(orders []model.Order) []*orderResponseDTO {
	res := []*orderResponseDTO{}
	for _, order := range orders {
		res = append(res, NewOrderResponse(order))
	}
	return res
}

func (gh *GophermartHandlers) Login(w http.ResponseWriter, r *http.Request) {
	var buf bytes.Buffer
	var auth authRequestDTO

	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		log.Println("error in reading body:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &auth); err != nil {
		log.Println("error in unmarshalling json:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !VerifyUser(context.Background(), auth.Login, auth.Password, gh.GmStorage) {
		w.WriteHeader(http.StatusUnauthorized) // неверная пара логин/пароль
		return
	}
	token, err := jwt.GenerateJwtToken(gh.SecretKey, auth.Login)
	if err != nil {
		log.Println("error in generating token:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.Header().Set("Authorization", "Bearer "+token)

	w.WriteHeader(http.StatusOK)
}

// Хендлер доступен только авторизованному пользователю
func (gh *GophermartHandlers) AddOrder(w http.ResponseWriter, r *http.Request) {
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
		log.Println("error in reading body:", err)
		w.WriteHeader(http.StatusBadRequest) // неверный формат запроса
		return
	}

	numOrder := buf.String()
	if !luhn.CheckLuhn(numOrder) {
		w.WriteHeader(http.StatusUnprocessableEntity) // неверный формат номера заказа
		return
	}

	log.Println("checked by Luhn numOrder:", numOrder)

	orderExists, err := gh.GmStorage.OrderExists(context.Background(), numOrder)
	if err != nil {
		log.Println("error in order exists:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !orderExists {
		err = gh.GmStorage.AddNewOrder(context.Background(), userLogin, numOrder)
		if err != nil {
			log.Println("error in adding order:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted) // новый номер заказа принят в обработку
		return
	} else {
		customerFromModel, err := gh.GmStorage.GetUserByOrder(context.Background(), numOrder)
		customer := NewCustomer(customerFromModel)

		if err != nil {
			log.Println("error in getting user by order:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if userLogin == customer.Login {
			w.WriteHeader(http.StatusOK) // номер заказа уже был загружен этим пользователем
			return
		}
		w.WriteHeader(http.StatusConflict) // номер заказа уже был загружен другим пользователем
		return
	}
}

// Хендлер доступен только авторизованному пользователю
func (gh *GophermartHandlers) GetOrders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	userLogin := r.Header.Get("x-user")

	customer, err := gh.GmStorage.GetCustomerByLogin(context.Background(), userLogin)
	log.Println("user from get customer by login:", *customer)
	if err != nil {
		log.Println("error in getting user by login:", err)
		w.WriteHeader(http.StatusInternalServerError) // ошибка с БД
		return
	}
	if customer == nil {
		log.Println("customer == nil")
		w.WriteHeader(http.StatusNotFound) // это значит, нет такого пользователя
		return
	}

	orders, err := gh.GmStorage.GetOrdersByUser(context.Background(), customer.ID)
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

	resp, err := json.Marshal(NewOrderListResponse(orders))
	if err != nil {
		log.Println("error in marshalling json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(resp)
	if err != nil {
		log.Println("error in writing resp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
}

// Хендлер доступен только авторизованному пользователю
func (gh *GophermartHandlers) GetBalance(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	userLogin := r.Header.Get("x-user")

	customer, err := gh.GmStorage.GetCustomerByLogin(context.Background(), userLogin)
	if err != nil {
		log.Println("error in getting customer by login:", err)
		w.WriteHeader(http.StatusInternalServerError) // ошибка с БД
		return
	}
	if customer == nil {
		w.WriteHeader(http.StatusNotFound) // нет такого пользователя
		return
	}
	pointsAccrual, err := gh.GmStorage.GetAccrualPoints(context.Background(), customer.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Println("error in getting accrual points:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pointsWithdrawal, err := gh.GmStorage.GetWithdrawalPoints(context.Background(), customer.ID)
	if err != nil && err != sql.ErrNoRows {
		log.Println("error in getting withdrawal points:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	points := pointsResponseDTO{
		Current:   pointsAccrual - pointsWithdrawal,
		Withdrawn: pointsWithdrawal,
	}

	resp, err := json.Marshal(points)
	if err != nil {
		log.Println("error in marshalling json:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(resp)
	if err != nil {
		log.Println("error in writing resp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Хендлер доступен только авторизованному пользователю
func (gh *GophermartHandlers) PostWithdraw(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	userLogin := r.Header.Get("x-user")

	customer, err := gh.GmStorage.GetCustomerByLogin(context.Background(), userLogin)
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
	var wreq withdrawRequest

	// читаем тело запроса
	_, err = buf.ReadFrom(r.Body)
	if err != nil {
		log.Println("error in reading body:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &wreq); err != nil {
		log.Println("error in unmarshalling json:", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if !luhn.CheckLuhn(wreq.OrderNumber) {
		w.WriteHeader(http.StatusUnprocessableEntity) // неверный номер заказа
		return
	}

	sumInAccount, err := gh.GmStorage.GetAccrualPoints(context.Background(), customer.ID)
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

	err = gh.GmStorage.Withdraw(context.Background(), wreq.OrderNumber, wreq.Sum, customer.ID)
	if err != nil {
		log.Printf("error %v in withdrawing points of user %d", err, customer.ID)
		w.WriteHeader(http.StatusInternalServerError) // внутренняя ошибка сервера
		return
	}
	w.WriteHeader(http.StatusOK)
}

// Хендлер доступен только авторизованному пользователю
func (gh *GophermartHandlers) GetWithdrawals(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	w.Header().Set("Content-Type", "application/json")

	userLogin := r.Header.Get("x-user")

	customer, err := gh.GmStorage.GetCustomerByLogin(context.Background(), userLogin)
	if err != nil {
		log.Printf("error %v in getting user by login %s", err, userLogin)
		w.WriteHeader(http.StatusInternalServerError) // ошибка с БД
		return
	}
	if customer == nil {
		w.WriteHeader(http.StatusNotFound) // это значит, нет такого пользователя
		return
	}

	transactions, err := gh.GmStorage.WithdrawalsByUser(context.Background(), customer.ID)
	if err != nil {
		log.Printf("error %v in getting withdrawals of user %d", err, customer.ID)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(transactions) == 0 {
		w.WriteHeader(http.StatusNoContent) // 204 — нет ни одного списания
		return
	}

	resp, err := json.Marshal(NewWithdrawalTxListResponse(transactions))
	if err != nil {
		log.Println("error in marshalling txs", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(resp)
	if err != nil {
		log.Println("error in writing resp:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

func (gh *GophermartHandlers) RegisterCustomer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}
	var buf bytes.Buffer
	var customer customerDTO

	// читаем тело запроса
	_, err := buf.ReadFrom(r.Body)
	if err != nil {
		log.Println("error in reading body:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &customer); err != nil {
		log.Println("error in unmarshalling json:", err)
		w.WriteHeader(http.StatusBadRequest) // неверный формат запроса
		return
	}

	err = gh.GmStorage.AddUser(context.Background(), customer.Login, customer.Password)
	if err != nil {
		if model.IsUserExistsErr(err) {
			log.Printf("error %v in registering user %s", err, customer.Login)
			w.WriteHeader(http.StatusConflict)
			return
		}
		log.Println("error in adding user:", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	token, err := jwt.GenerateJwtToken(gh.SecretKey, customer.Login)
	if err != nil {
		log.Printf("error %v in generating token for login %s", err, customer.Login)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	// автоматическая аутентификация пользователя после успешной регистрации
	w.Header().Set("Authorization", "Bearer "+token)

	w.WriteHeader(http.StatusOK)
}

// VerifyUser — функция, которая выполняет аутентификацию и авторизацию пользователя
// user — логин пользователя, pass — пароль, permission — необходимая привилегия.
// если пользователь ввел правильные данные, и у него есть необходимая привилегия — возвращаем true, иначе — false
func VerifyUser(ctx context.Context, login string, pass string, gmStorage *model.GophermartStorage) bool {
	// получаем хеш пароля
	hashedPassword := sha256.Sum256([]byte(pass))
	hashStringPassword := hex.EncodeToString(hashedPassword[:])
	log.Println(hashStringPassword)

	// проверяем введенные данные
	cust, err := gmStorage.GetCustomerByLogin(ctx, login)
	if err != nil {
		log.Printf("Error in authorization %s", cust.Login)
		return false
	}

	log.Println(cust.Password)
	return cust.Password == pass
}
