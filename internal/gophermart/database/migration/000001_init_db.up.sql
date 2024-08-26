create table customer (id serial primary key, 
	login varchar(100) not null,
	password varchar(255) not null,
	created_at timestamp not null default now(),
	unique(login));

create table "order"
	(id serial primary key, 
    customer_id integer, 
	number text,
	status varchar(30) not null, 
	created_at timestamp not null default now(),
	foreign key (customer_id) references customer (id),
	unique(number, customer_id));

create table loyalty_system
	(id serial primary key,
	customer_id integer, 
	order_id integer,
	points double precision,
	transacton varchar(30), 
	created_at timestamp not null default now(),
	unique(customer_id, order_id),
	foreign key (customer_id) references customer (id),
	foreign key (order_id) references "order" (id));

create table product
	(id serial primary key,
	title varchar(60), 
	price integer);

create table order_product
	(id serial primary key,
	order_id integer references "order" (id), 
	product_id integer references product (id),
	amount integer);