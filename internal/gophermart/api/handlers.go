package api

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
	"github.com/adettelle/loyalty-system/pkg/mware/security"
	"github.com/adettelle/loyalty-system/pkg/validation/luhn"
)

// type Handler struct {
// 	Storager Storager
// 	Config   *config.Config
// }

// type Storager interface {
// 	AddOrder(w http.ResponseWriter, r *http.Request)
// 	GetOrders(w http.ResponseWriter, r *http.Request)
// 	GetBalance(w http.ResponseWriter, r *http.Request)
// 	PostWithdraw(w http.ResponseWriter, r *http.Request)
// 	GetWithdrawals(w http.ResponseWriter, r *http.Request)
// }

type DBStorage struct {
	Ctx       context.Context
	DB        *sql.DB
	SecretKey []byte // []byte("my_secret_key")
}

type Auth struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type TransactionWResponse struct {
	OrderNumber string    `json:"order"`
	Points      float64   `json:"sum"`
	CreatedAt   time.Time `json:"created_at"`
}

type OrderResponse struct {
	Number        string    `json:"number"`
	Status        string    `json:"status"`
	Points        *float64  `json:"points,omitempty"`
	Accrual       float64   `json:"accrual"`              //,omitempty
	Withdrawal    float64   `json:"withdrawal,omitempty"` //
	CreatedAt     time.Time `json:"uploaded_at"`          // created_at
	SumToWithdraw float64   `json:"sum,omitempty"`
}

type Customer struct {
	Login    string `json:"login"`
	Password string `json:"password"`
}

type PointsResponse struct {
	Current   float64 `json:"current"`
	Withdrawn float64 `json:"withdrawn"`
}

// type OrdersListResponse struct {
// 	Orders []OrderResponse `json:"orders"`
// }

func NewOrderResponse(order model.Order) OrderResponse {
	res := OrderResponse{
		Number:    order.Number,
		Status:    order.Status,
		CreatedAt: order.CreatedAt, // Формат даты — RFC3339
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
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	// 400 — неверный формат запроса; ????????????????????????

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

	orderExists, err := model.OrderExists(numOrder, s.DB, s.Ctx) // что делать с ошибкой?????????
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	if !orderExists {
		log.Println("Writing to DB")

		sqlStatement := `insert into "order" (customer_id, number, status)
			values ((select id from customer where login = $1), $2, $3);`

		_, err = s.DB.ExecContext(s.Ctx, sqlStatement, userLogin, numOrder, model.StatusNew)
		if err != nil {
			log.Println("error in adding order:", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusAccepted) // новый номер заказа принят в обработку
	} else {
		idUser, err := model.GetUserByOrder(numOrder, s.DB, s.Ctx)
		if err != nil {
			// правильно ли выбрала тип ошибки?????????
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		loginToCheck, err := model.GetLoginByID(idUser, s.DB, s.Ctx)
		if err != nil {
			// правильно ли выбрала тип ошибки?????????
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if userLogin == loginToCheck {
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

	orders, err := model.GetOrdersByUser(customer.ID, s.DB, s.Ctx)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if len(orders) == 0 {
		w.WriteHeader(http.StatusNoContent) // 204 — нет данных для ответа
		return
	}

	resp, err := json.Marshal(NewOrderListResponse(orders)) // NewOrderListResponse(orders)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	_, err = w.Write(resp)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusOK)
}

// type PointsListResponse struct {
// 	Points []PointsResponse `json:"points"`
// }

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
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	pointsWithdrawal, err := model.GetWithdrawalPoints(customer.ID, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	points := PointsResponse{
		Current:   pointsAccrual,
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

// type OrderResponse struct {
// 	OrderNumber string `json:"order"`
// 	Sum         int    `json:"sum"`
// }

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
	var orderResp OrderResponse

	// читаем тело запроса
	_, err = buf.ReadFrom(r.Body)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest) // неверный формат запроса
		return
	}

	if err := json.Unmarshal(buf.Bytes(), &orderResp); err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	fmt.Println(orderResp.Number)
	fmt.Println(orderResp.SumToWithdraw)

	if !luhn.CheckLuhn(orderResp.Number) {
		w.WriteHeader(http.StatusUnprocessableEntity) // неверный номер заказа
		return
	}

	sumInAccount, err := model.GetAccrualPoints(customer.ID, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	log.Println("sumInAccount:", sumInAccount)
	log.Println("orderResp.Sum:", orderResp.SumToWithdraw)

	if sumInAccount < orderResp.SumToWithdraw {
		w.WriteHeader(http.StatusPaymentRequired) // на счету недостаточно средств
		return
	}

	err = model.Withdraw(orderResp.Number, orderResp.SumToWithdraw, s.DB, s.Ctx)
	if err != nil {
		log.Println(err)
		w.WriteHeader(http.StatusInternalServerError) // внутренняя ошибка сервера
		return
	}
	w.WriteHeader(http.StatusOK)
}

// type TransactionWListResponse struct {
// 	TransactionWs []TransactionWResponse `json:"transactions_w"`
// }

func NewTransactionWResponse(transaction model.TransactionW) TransactionWResponse {
	return TransactionWResponse{
		OrderNumber: transaction.OrderNumber,
		Points:      transaction.Points,
		CreatedAt:   transaction.CreatedAt, // Формат даты — RFC3339
	}
}
func NewTransactionWListResponse(transactions []model.TransactionW) []TransactionWResponse {
	res := []TransactionWResponse{}
	for _, transaction := range transactions {
		res = append(res, NewTransactionWResponse(transaction))
	}
	return res
}

// func NewTransactionWListResponse(transactions []model.TransactionW) TransactionWListResponse {
// 	res := TransactionWListResponse{
// 		TransactionWs: []TransactionWResponse{},
// 	}
// 	for _, transaction := range transactions {
// 		res.TransactionWs = append(res.TransactionWs, NewTransactionWResponse(transaction))
// 	}
// 	return res
// }

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

// 409 — логин уже занят ????????????????????????
// После успешной регистрации должна происходить автоматическая аутентификация пользователя ?????????
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
	// fmt.Println(customer.Login)
	// fmt.Println(customer.Password)
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
	w.Header().Set("Authorization", "Bearer "+token)

	w.WriteHeader(http.StatusOK)
}

/*
func CheckToken(signedToken string) bool {
	// создаём секретный ключ для подписи
	secret := []byte("my_secret_key")

	// второй аргумент — функция, которая просто возвращает секретный ключ
	// чтобы было понятней, мысленно вместо функции подставьте возвращаемое значение
	jwtToken, err := jwt.Parse(signedToken, func(t *jwt.Token) (interface{}, error) {
		// секретный ключ для всех токенов одинаковый, поэтому просто возвращаем его
		return secret, nil
	})
	if err != nil {
		fmt.Printf("Failed to parse token: %s\n", err)
		return false
	}
	if jwtToken.Valid {
		log.Println("Токен валиден")
		return true
	} else {
		log.Println("Токен невалиден")
		return false
	}
}
*/
/*
func CheckRequestToken(w http.ResponseWriter, r *http.Request) error {
	// получаем http header вида 'Bearer {jwt}'
	authHeaderValue := r.Header.Get("Authorization")
	log.Println("authHeaderValue:", authHeaderValue)
	if authHeaderValue == "" {
		w.WriteHeader(http.StatusUnauthorized) // пользователь не аутентифицирован
		return fmt.Errorf("error in CheckRequestToken")
	}

	// проверяем доступы
	if authHeaderValue != "" {
		bearerToken := strings.Split(authHeaderValue, " ")
		log.Println("bearerToken:", bearerToken[1])
		if len(bearerToken) == 2 {
			login, ok := security.VerifyToken(bearerToken[1])
			if !ok {
				w.WriteHeader(http.StatusUnauthorized) // пользователь не аутентифицирован
				return fmt.Errorf("error in CheckRequestToken")
			} else {
				r.Header.Set("x-user", login) // x - кастомные хэддеры приянто называть с перфиксом x
				// userLogin = login
			}
		}
	}
	return nil
}
*/
