package database

import (
	"context"
	"database/sql"
	"log"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func CreateTable(db *sql.DB, ctx context.Context) error { // user
	sqlStCustomer := `create table if not exists customer 
		(id serial primary key, 
		first_name varchar(30), 
		last_name varchar(30),
		email varchar(100),
		phone varchar(30), 
		login varchar(100) not null,
		password varchar(255) not null,
		created_at timestamp not null default now(),
		unique(phone, email));`

	_, err := db.ExecContext(ctx, sqlStCustomer)
	if err != nil {
		return err
	}

	statusType := `create type status_type_enum as enum ('new', 'processing', 'invalid', 'processed');`
	_, err = db.ExecContext(ctx, statusType) // , model.StatusNew, model.StatusProcessing, model.StatusInvalid, model.StatusProcessed)
	if err != nil {
		return err
	}

	sqlStOrder := `create table if not exists "order"
		(id serial primary key,
		customer_id integer, 
		number text,
		status status_type_enum not null, 
		created_at timestamp not null default now(),
		foreign key (customer_id) references customer (id),
		unique(number, customer_id));`

	_, err = db.ExecContext(ctx, sqlStOrder)
	if err != nil {
		return err
	}

	// начисления и списания
	transactionType := `create type transaction_type_enum as enum ('accrual', 'withdrawal');`
	_, err = db.ExecContext(ctx, transactionType) // , model.TransactionAccrual, model.TransactionWithdrawal)
	if err != nil {
		return err
	}

	sqlStLoyalty := `create table if not exists loyalty_system
		(id serial primary key,
		customer_id integer, 
		order_id integer,
		points double precision,
		transacton transaction_type_enum, 
		created_at timestamp not null default now(),
		unique(customer_id, order_id),
		foreign key (customer_id) references customer (id),
		foreign key (order_id) references "order" (id));`

	_, err = db.ExecContext(ctx, sqlStLoyalty)
	if err != nil {
		return err
	}

	sqlStProduct := `create table if not exists product
		(id serial primary key,
		title varchar(60), 
		price integer);`

	_, err = db.ExecContext(ctx, sqlStProduct)
	if err != nil {
		return err
	}

	sqlStOrderProduct := `create table if not exists order_product
		(id serial primary key,
		order_id integer references "order" (id), 
		product_id integer references product (id),
		amount integer);`

	_, err = db.ExecContext(ctx, sqlStOrderProduct)
	if err != nil {
		return err
	}

	rewardType := `create type reward_type_enum as enum ('percent', 'points');`
	_, err = db.ExecContext(ctx, rewardType) //, model.RewardTypePercent, model.RewardTypePoints)
	if err != nil {
		return err
	}

	sqlStReward := `create table if not exists reward
		(id serial primary key,
		title varchar(60), 
		product_id integer references product (id),
		description varchar(255),
		reward_type reward_type_enum not null);`

	_, err = db.ExecContext(ctx, sqlStReward)
	if err != nil {
		return err
	}

	// sqlStRewardSystem := `create table if not exists reward_system
	// 	(id serial primary key,
	// 	order_id integer references "order" (id),
	// 	points double precision);`

	// _, err = db.ExecContext(ctx, sqlStRewardSystem)
	// if err != nil {
	// 	return err
	// }

	return nil
}

func Connect(dbParams string) (*sql.DB, error) {
	log.Println("Connecting to DB", dbParams)
	db, err := sql.Open("pgx", dbParams)
	if err != nil {
		return nil, err
	}
	return db, nil
}
