package main

import (
	"database/sql"
	"fmt"
	"log"

	_ "github.com/lib/pq"
)

type Storage interface {
	CreateAccount(*Account) error
	DeleteAccount(int) error
	UpdateAccount(*Account) error
	GetAccounts() ([]*Account, error)
	GetAccountByID(int) (*Account, error)
}

type PostgresStore struct {
	db *sql.DB
}

func NewPostgresStore() (*PostgresStore, error) {
	connStr := "user=postgres dbname=gobank password=miawallace sslmode=disable"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		return nil, err
	}
	if err = db.Ping(); err != nil {
		return nil, err
	}
	return &PostgresStore{
		db: db,
	}, nil
}

func (s *PostgresStore) Init() error {
	return s.createAccountTable()
}

func (s *PostgresStore) createAccountTable() error {
	query := `Create table if not exists account (
		id serial primary key,
		first_name varchar(50),
		last_name varchar(50),
		number serial,
		balance serial,
		created_at timestamp
	)`

	_, err := s.db.Exec(query)

	return err
}

func (s *PostgresStore) CreateAccount(a *Account) error {
	query := `insert into account (first_name, last_name, number, balance, created_at) values($1, $2, $3, $4, $5) returning id`
	id := 0
	err := s.db.QueryRow(query, a.FirstName, a.LastName, a.Number, a.Balance, a.CreatedAt).Scan(&id)
	if err != nil {
		return err
	}
	a.ID = id
	return nil
}
func (s *PostgresStore) UpdateAccount(a *Account) error {

	return nil
}
func (s *PostgresStore) DeleteAccount(id int) error {
	_, err := s.db.Query("delete from account where id = $1", id)
	if err != nil {
		return fmt.Errorf("Cound not delete account with id %d", id)
	}
	return nil
}
func (s *PostgresStore) GetAccountByID(id int) (*Account, error) {
	log.Println("account id: ", id)
	rows, err := s.db.Query("SELECT * from account where id = $1", id)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		return scanIntoAccount(rows)
	}
	return nil, fmt.Errorf("account %d not found", id)
}
func (s *PostgresStore) GetAccounts() ([]*Account, error) {
	resp, err := s.db.Query("select * from account")
	if err != nil {
		return nil, err
	}
	accounts := []*Account{}
	for resp.Next() {
		account := new(Account)
		account, err = scanIntoAccount(resp)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}
	return accounts, nil
}

func scanIntoAccount(rows *sql.Rows) (*Account, error) {
	account := Account{}
	err := rows.Scan(
		&account.ID,
		&account.FirstName,
		&account.LastName,
		&account.Number,
		&account.Balance,
		&account.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &account, nil
}
