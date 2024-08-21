package model

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const (
	StatusNew             = "NEW"
	StatusProcessing      = "PROCESSING"
	StatusInvalid         = "INVALID"
	StatusProcessed       = "PROCESSED"
	TransactionAccrual    = `accrual`
	TransactionWithdrawal = `withdrawal`
	// RewardTypePercent     = `percent`
	// RewardTypePoints      = `points`
)

type Order struct {
	ID          int
	Number      string
	Status      string
	Points      float64
	Transaction *string
	CreatedAt   time.Time
}

type Customer struct {
	ID       int
	Login    string
	Password string
}

// транзакция списания
type TxWithdraw struct {
	OrderNumber string
	Points      float64
	CreatedAt   time.Time
}

type GophermartStorage struct {
	Ctx context.Context
	DB  *sql.DB
}

func NewGophermartStorage(db *sql.DB, ctx context.Context) *GophermartStorage {
	return &GophermartStorage{
		Ctx: ctx,
		DB:  db,
	}
}

type UserExistsErr struct {
	login string
}

func (ue *UserExistsErr) Error() string {
	return fmt.Sprintf("user %s already exists", ue.login)
}

func NewUserExistsErr(login string) *UserExistsErr {
	return &UserExistsErr{
		login: login,
	}
}

func IsUserExistsErr(err error) bool {
	var customErr *UserExistsErr
	return errors.As(err, &customErr)
}

// GetUserByOrder возвращает юзера и ошибку
func (gs *GophermartStorage) GetUserByOrder(numOrder string) (*Customer, error) {
	sqlSt := `select c.id, c.login, c."password" from "order" ord
		inner join customer c on c.id = ord.customer_id 
		where "number" = $1;`
	row := gs.DB.QueryRowContext(gs.Ctx, sqlSt, numOrder)

	var customer Customer
	err := row.Scan(&customer.ID, &customer.Login, &customer.Password)

	return &customer, err
}

func (gs *GophermartStorage) OrderExists(numOrder string) (bool, error) {
	sqlSt := `select count(id) > 0 as order_exists from "order" where "number" = $1;`
	row := gs.DB.QueryRowContext(gs.Ctx, sqlSt, numOrder)

	var ordExists bool

	err := row.Scan(&ordExists)
	log.Println("error in OrderExists:", err)
	log.Println("ordExists:", ordExists)
	return ordExists, err
}

func (gs *GophermartStorage) GetOrdersByUser(userID int) ([]Order, error) {
	orders := make([]Order, 0)

	sqlSt := `select ord.id, "number", status, coalesce(ls.points, 0), ls.transacton, ord.created_at 
		from "order" ord
		left join loyalty_system ls 
		on ls.order_id = ord.id
		where ord.customer_id = $1
		order by ord.created_at;`

	rows, err := gs.DB.QueryContext(gs.Ctx, sqlSt, userID)
	if err != nil || rows.Err() != nil {
		log.Println("error: ", err)
		return nil, err
	}
	defer rows.Close()

	// пробегаем по всем записям
	for rows.Next() {
		var ord Order
		err := rows.Scan(&ord.ID, &ord.Number, &ord.Status, &ord.Points, &ord.Transaction, &ord.CreatedAt)
		if err != nil {
			log.Println("error: ", err)
			return nil, err
		}

		orders = append(orders, ord)
	}
	return orders, nil
}

// GetAccrualPoints показывает количество набранных баллов пользователя
func (gs *GophermartStorage) GetAccrualPoints(userID int) (float64, error) {
	sqlSt := `select coalesce (sum(points), 0) from loyalty_system 
		where customer_id = $1 and transacton = $2;`

	row := gs.DB.QueryRowContext(gs.Ctx, sqlSt, userID, TransactionAccrual)

	var pointsAccrual float64

	err := row.Scan(&pointsAccrual)
	if err != nil {
		log.Printf("Error %v in getting balance of user %d", err, userID)

		return 0, err
	}

	return pointsAccrual, nil
}

// GetWithdrawalPoints показывает количество потраченных баллов пользователя
func (gs *GophermartStorage) GetWithdrawalPoints(userID int) (float64, error) {
	sqlSt := `select coalesce (sum(points), 0) from loyalty_system
		where customer_id = $1 and transacton = $2;`

	row := gs.DB.QueryRowContext(gs.Ctx, sqlSt, userID, TransactionWithdrawal)

	var pointsWithdrawal float64

	err := row.Scan(&pointsWithdrawal)
	if err != nil {
		log.Printf("Error in getting balance of user %d", userID)
		return 0, err
	}

	return pointsWithdrawal, nil
}

// Withdraw списывает баллы sum с номера счета order у зарегистрированного пользователя
func (gs *GophermartStorage) Withdraw(order string, sum float64, userID int) error {
	sqlNewOrder := `insert into "order" (customer_id, "number", status)
		values ($1, $2, $3) returning id;`

	row := gs.DB.QueryRowContext(gs.Ctx, sqlNewOrder, userID, order, StatusNew)

	var orderID int

	err := row.Scan(&orderID)
	if err != nil {
		log.Printf("error %v in inserting new order %s by withdrawal %f", err, order, sum)
		return err
	}

	sqlSt := `insert into loyalty_system (customer_id, order_id, points, transacton)
		values ($1, $2, $3, $4);`

	_, err = gs.DB.ExecContext(gs.Ctx, sqlSt, userID, orderID, sum, TransactionWithdrawal)
	if err != nil {
		log.Printf("error %v in inserting new withdrawal %f", err, sum)
		return err
	}

	return nil
}

// WithdrawalsByUser показывает все транзакции с выводом средств
func (gs *GophermartStorage) WithdrawalsByUser(userID int) ([]TxWithdraw, error) {
	transactions := make([]TxWithdraw, 0)
	sqlSt := `select ord."number", ls.points, ls.created_at 
		from loyalty_system ls 
		join "order" ord
		on ord.id = ls.order_id
		where ls.transacton = $1 and ls.customer_id = $2
		order by created_at desc;`

	rows, err := gs.DB.QueryContext(gs.Ctx, sqlSt, TransactionWithdrawal, userID)
	if err != nil || rows.Err() != nil {
		return nil, err
	}
	defer rows.Close()

	// пробегаем по всем записям
	for rows.Next() {
		var tr TxWithdraw
		err := rows.Scan(&tr.OrderNumber, &tr.Points, &tr.CreatedAt)
		if err != nil || rows.Err() != nil {
			return nil, err
		}

		transactions = append(transactions, tr)
	}
	return transactions, nil
}

func (gs *GophermartStorage) GetCustomerByLogin(login string) (*Customer, error) {
	sqlSt := `select id, login, password from customer where login = $1;`

	row := gs.DB.QueryRowContext(gs.Ctx, sqlSt, login)

	var customer Customer

	err := row.Scan(&customer.ID, &customer.Login, &customer.Password)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, nil // считаем, что это не ошибка, просто не нашли пользователя
		}
		return nil, err
	}
	return &customer, nil
}

// регистрация пользователя
func (gs *GophermartStorage) AddUser(login string, password string) error {
	sqlUser := `select count(*) > 0 from customer where login = $1 limit 1;`
	row := gs.DB.QueryRowContext(gs.Ctx, sqlUser, login)

	// переменная для чтения результата
	var userEsists bool

	err := row.Scan(&userEsists)

	if err != nil {
		return err
	}
	if userEsists {
		return NewUserExistsErr(login)
	}

	sqlSt := `insert into customer (login, "password") values ($1, $2);`

	_, err = gs.DB.ExecContext(gs.Ctx, sqlSt, login, password)
	if err != nil {
		log.Println("error in registering user:", err)
		return err
	}
	log.Println("Registered")
	return nil
}

// GetAllProcessingOrders находит все заказы состатусом new и меняет их статус на processing
func (gs *GophermartStorage) GetAllProcessingOrders() ([]Order, error) {
	orders := make([]Order, 0)

	sqlSt := `update "order" set status = 'PROCESSING' where status = 'NEW' returning id, "number", status;`

	rows, err := gs.DB.QueryContext(gs.Ctx, sqlSt)
	if err != nil || rows.Err() != nil {
		return nil, err
	}
	defer rows.Close()

	for rows.Next() {
		var ord Order
		err := rows.Scan(&ord.ID, &ord.Number, &ord.Status)
		if err != nil {
			return nil, err
		}
		orders = append(orders, ord)
	}
	return orders, nil
}

func (gs *GophermartStorage) UpdateOrderStatus(status string, number string) error {
	sqlSt := `update "order" set status = $1 where "number" = $2;`

	_, err := gs.DB.ExecContext(gs.Ctx, sqlSt, status, number)

	if err != nil {
		return err
	}
	log.Println("Status updated")
	return nil
}

func (gs *GophermartStorage) UpdateAccrualPoints(accrual float64, number string) error {
	sqlSt := `insert into loyalty_system (customer_id, order_id, points, transacton)
		values ((select customer_id from "order" where "number" = $1), 
		(select id from "order" where "number" = $1), $2, $3);
`

	_, err := gs.DB.ExecContext(gs.Ctx, sqlSt, number, accrual, TransactionAccrual)
	if err != nil {
		return err
	}
	log.Println("Points have been accrued")
	return nil
}

func (gs *GophermartStorage) AddNewOrder(userLogin string, numOrder string) error {
	log.Println("Writing to DB")

	sqlStatement := `insert into "order" (customer_id, number, status)
			values ((select id from customer where login = $1), $2, $3);`

	_, err := gs.DB.ExecContext(gs.Ctx, sqlStatement, userLogin, numOrder, StatusNew)
	if err != nil {
		return err
	}
	log.Println("Order have been added")
	return nil
}
