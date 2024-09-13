package main

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	_ "modernc.org/sqlite"
)

type Customer struct {
	ID        int64  `json:"id"` // incremental id
	Name      string `json:"name"`
	DOB       string `json:"dob"`
	Email     string `json:"email"`
	Contact   string `json:"contact"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

type CustomerDetails struct {
	Name    string `json:"name"`
	DOB     string `json:"dob"`
	Email   string `json:"email"`
	Contact string `json:"contact"`
}

type GetListingResponse struct {
	Data       []Customer `json:"data"`
	TotalPages int        `json:"total_pages"`
}

type ApiResponse[T any] struct {
	Data T `json:"data"`
}

// ConvertInt converts string to int, defaults to 0 if conversion fails
func ConvertInt(s string) int {
	if s == "" {
		return 0
	}

	i, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}

	return i
}

func main() {
	// initialize sqlite database connection
	db, err := sql.Open("sqlite", "./database.db")
	if err != nil {
		panic(err)
	}

	// create the table
	err = create_table(db)
	if err != nil {
		panic(err)
	}

	mux := http.NewServeMux()

	// health check api
	mux.HandleFunc("GET /", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte("Service is up!"))
	})

	// register the customer
	mux.HandleFunc("POST /customers", func(w http.ResponseWriter, r *http.Request) {
		// receive the request in json body
		var req CustomerDetails
		err := json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		// create the customer
		customer, err := create_customer(db, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := ApiResponse[Customer]{
			Data: *customer,
		}

		response_str, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		// return response
		w.Header().Set("Content-Type", "application/json")
		w.Write(response_str)
	})

	// update the customer
	mux.HandleFunc("PUT /customers/{id}", func(w http.ResponseWriter, r *http.Request) {
		id_str := r.PathValue("id")
		if id_str == "" {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}
		id, err := strconv.ParseInt(id_str, 10, 64)
		if err != nil {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}

		var req CustomerDetails
		err = json.NewDecoder(r.Body).Decode(&req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		customer, err := update_customer(db, id, req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := ApiResponse[Customer]{
			Data: *customer,
		}

		response_str, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		} else {
			w.Header().Set("Content-Type", "application/json")
			w.Write(response_str)
		}
	})

	// get customers
	mux.HandleFunc("GET /customers/{id}", func(w http.ResponseWriter, r *http.Request) {
		// get the id from the url
		id_str := r.PathValue("id")
		if id_str == "" {
			http.Error(w, "Invalid id", http.StatusBadRequest)
		}
		// convert to int64
		id, err := strconv.ParseInt(id_str, 10, 64)
		if err != nil {
			http.Error(w, "Invalid id", http.StatusBadRequest)
			return
		}

		// try to find the customer
		customer, err := get_customer(db, id)
		if err != nil && err.Error() == "Customer not found" {
			http.Error(w, "Customer not found", http.StatusNotFound)
			return
		}

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := ApiResponse[Customer]{
			Data: *customer,
		}

		response_str, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// return response
		w.Header().Set("Content-Type", "application/json")
		w.Write(response_str)
	})

	mux.HandleFunc("GET /customers", func(w http.ResponseWriter, r *http.Request) {
		// get params for pagination
		page := ConvertInt(r.URL.Query().Get("page"))
		limit := ConvertInt(r.URL.Query().Get("limit"))

		if page == 0 {
			page = 1
		}

		if limit == 0 {
			limit = 10
		}

		result, err := get_customers(db, (page-1)*limit, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		total_records, err := get_total_customers(db)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		response := ApiResponse[GetListingResponse]{
			Data: GetListingResponse{
				Data:       result,
				TotalPages: total_records / limit,
			},
		}
		response_str, err := json.Marshal(response)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}

		// return response
		w.Header().Set("Content-Type", "application/json")
		w.Write(response_str)
	})

	println("Server is running on port 3000")
	http.ListenAndServe(":3000", mux)
}

// #region Database
func create_table(db *sql.DB) error {
	sql_table := `
	CREATE TABLE IF NOT EXISTS customers (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT,
		dob TEXT,
		email TEXT,
		contact TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	);
	`

	_, err := db.Exec(sql_table)
	return err
}

func create_customer(db *sql.DB, input CustomerDetails) (*Customer, error) {
	create_record := `
	INSERT INTO customers (name, dob, email, contact)
	VALUES (?, ?, ?, ?);
	`

	result, err := db.Exec(create_record, input.Name, input.DOB, input.Email, input.Contact)
	if err != nil {
		return nil, err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, err
	}

	// get the customer
	customer, err := get_customer(db, id)
	if err != nil {
		return nil, err
	}

	return customer, nil
}

func update_customer(db *sql.DB, i int64, input CustomerDetails) (*Customer, error) {
	update_record := `
	UPDATE customers
	SET name = ?, dob = ?, email = ?, contact = ?, updated_at = CURRENT_TIMESTAMP
	WHERE id = ?;
	`

	_, err := db.Exec(update_record, input.Name, input.DOB, input.Email, input.Contact, i)
	if err != nil {
		return nil, err
	}

	updated_customer, err := get_customer(db, i)
	if err != nil {
		return nil, err
	}

	return updated_customer, nil
}

func get_customer(db *sql.DB, i int64) (*Customer, error) {
	get_record := `
	SELECT id, name, dob, email, contact, created_at, updated_at
	FROM customers
	WHERE id = ?;
	`

	var customer Customer
	err := db.QueryRow(get_record, i).Scan(&customer.ID, &customer.Name, &customer.DOB, &customer.Email, &customer.Contact, &customer.CreatedAt, &customer.UpdatedAt)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, errors.New("Customer not found")
		}
		return nil, err
	}

	return &customer, nil
}

func get_customers(db *sql.DB, offset int, limit int) ([]Customer, error) {
	get_records := `
	SELECT id, name, dob, email, contact, created_at, updated_at
	FROM customers
	LIMIT ? OFFSET ?;
	`

	rows, err := db.Query(get_records, limit, offset)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	var customers []Customer = []Customer{}
	for rows.Next() {
		var customer Customer
		err = rows.Scan(&customer.ID, &customer.Name, &customer.DOB, &customer.Email, &customer.Contact, &customer.CreatedAt, &customer.UpdatedAt)
		if err != nil {
			return nil, err
		}

		customers = append(customers, customer)
	}

	return customers, nil
}

func get_total_customers(db *sql.DB) (int, error) {
	get_records := `
	SELECT COUNT(*)
	FROM customers;
	`

	var count int
	err := db.QueryRow(get_records).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// #endregion
