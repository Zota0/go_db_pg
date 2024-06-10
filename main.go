package main

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/joho/godotenv"
	_ "github.com/lib/pq"
)

type Invoice struct {
	ID  *int    `json:"id,omitempty"`
	VAT *string `json:"vat,omitempty"`
}

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("ERROR: Loading .env,", err)
	}
	var auth string = os.Getenv("AUTH")
	var dbTable string = os.Getenv("DB_TABLE")

	http.HandleFunc("/", RootHandler)
	http.HandleFunc("/get", func(w http.ResponseWriter, r *http.Request) {
		GetInvoices(w, r, auth, dbTable)
	})
	http.HandleFunc("/del", func(w http.ResponseWriter, r *http.Request) {
		DeleteInvoice(w, r, auth, dbTable)
	})
	http.HandleFunc("/add", func(w http.ResponseWriter, r *http.Request) {
		AddInvoice(w, r, auth, dbTable)
	})
	http.HandleFunc("/upd", func(w http.ResponseWriter, r *http.Request) {
		UpdateInvoice(w, r, auth, dbTable)
	})

	log.Fatal(http.ListenAndServe(":9413", nil))
}

// Function to write JSON error responses
func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	resp := map[string]string{"msg": message}
	json.NewEncoder(w).Encode(resp)
}

func ConnectDB() (*sql.DB, error) {
	db, err := sql.Open("postgres", os.Getenv("DB_URI"))
	if err != nil {
		return nil, err
	}
	return db, nil
}

func RowsToJson(rows *sql.Rows) (string, error) {
	columns, err := rows.Columns()
	if err != nil {
		return "", fmt.Errorf("fetch error: %w", err)
	}

	var result []map[string]interface{}
	for rows.Next() {
		columnKey := make([]interface{}, len(columns))
		columnValues := make([]interface{}, len(columns))

		for i := range columnValues {
			columnKey[i] = &columnValues[i]
		}

		if err := rows.Scan(columnKey...); err != nil {
			return "", fmt.Errorf("failed to scan data: %w", err)
		}

		row := make(map[string]interface{})
		for i, colName := range columns {
			row[colName] = columnValues[i]
		}

		result = append(result, row)
	}

	json_data, err := json.Marshal(result)
	if err != nil {
		return "", fmt.Errorf("failed to marshal JSON: %w", err)
	}

	return string(json_data), nil
}

func RootHandler(w http.ResponseWriter, r *http.Request) {
	Head := w.Header()

	Head.Add("Content-Type", "application/json")
	Head.Add("Access-Control-Allow-Origin", "*")
	Head.Add("Access-Control-Allow-Methods", "GET")

	res := map[string]string{
		"status": "ready",
		"msg":    "Hello",
	}

	json_data, json_err := json.Marshal(res)
	if json_err != nil {
		http.Error(w, "Error marshalling JSON res", http.StatusInternalServerError)
		return
	}

	w.Write(json_data)
}

func GetInvoices(w http.ResponseWriter, r *http.Request, AUTH string, TABLE string) {
	Head := w.Header()
	db, db_err := ConnectDB()
	if db_err != nil {
		writeJSONError(w, "Error connecting to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	Head.Add("Content-Type", "application/json")
	Head.Add("Access-Control-Allow-Origin", "*")
	Head.Add("Access-Control-Allow-Methods", "GET")
	req_auth := r.Header.Get("Authorization")

	if req_auth != AUTH {
		writeJSONError(w, "Invalid auth", http.StatusUnauthorized)
		return
	}

	req := r.URL.Query()
	req_id := req.Get("id")
	req_what := req.Get("what")

	var rows *sql.Rows
	var err error

	if req_id != "" && req_what != "" {
		rows, err = db.Query(fmt.Sprintf("SELECT %s FROM %s WHERE id = $1", req_what, TABLE), req_id)
	} else if req_what != "" {
		rows, err = db.Query(fmt.Sprintf("SELECT %s FROM %s", req_what, TABLE))
	} else {
		writeJSONError(w, "Missing 'id' or 'what' parameter", http.StatusBadRequest)
		return
	}

	if err != nil {
		writeJSONError(w, "Error fetching data", http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		writeJSONError(w, "Error getting column names", http.StatusInternalServerError)
		return
	}

	var data []map[string]interface{}
	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))
		for i := range values {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			writeJSONError(w, "Error scanning data", http.StatusInternalServerError)
			return
		}

		row := make(map[string]interface{})
		for i, col := range columns {
			var v interface{}
			val := values[i]
			b, ok := val.([]byte)
			if ok {
				v = string(b)
			} else {
				v = val
			}
			row[col] = v
		}
		data = append(data, row)
	}

	jsonData, err := json.Marshal(data)
	if err != nil {
		writeJSONError(w, "Error marshalling data", http.StatusInternalServerError)
		return
	}

	w.Write([]byte(`{"msg": "ok", "data": `))
	w.Write(jsonData)
	w.Write([]byte(`}`))
}

func DeleteInvoice(w http.ResponseWriter, r *http.Request, AUTH string, TABLE string) {
	Head := w.Header()

	db, err := ConnectDB()
	if err != nil {
		writeJSONError(w, "Error connecting to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	Head.Add("Content-Type", "application/json")
	Head.Add("Access-Control-Allow-Origin", "*")
	Head.Add("Access-Control-Allow-Methods", "DELETE")

	req := r.URL.Query()
	req_auth := r.Header.Get("Authorization")
	req_id := req.Get("id")

	if req_auth != AUTH {
		writeJSONError(w, "Invalid auth", http.StatusUnauthorized)
		return
	}

	if req_id == "" {
		writeJSONError(w, "Invalid id", http.StatusBadRequest)
		return
	}

	result, err := db.Exec("DELETE FROM "+TABLE+" WHERE id = $1", req_id)
	if err != nil {
		writeJSONError(w, "Error deleting data", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		writeJSONError(w, "Error getting affected rows", http.StatusInternalServerError)
		return
	}

	response := fmt.Sprintf("{\"msg\": \"ok\", \"affected\": %d}", rowsAffected)
	w.Write([]byte(response))
}

func AddInvoice(w http.ResponseWriter, r *http.Request, AUTH string, TABLE string) {
	Head := w.Header()

	db, db_err := ConnectDB()
	if db_err != nil {
		writeJSONError(w, "Error connecting to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	Head.Add("Content-Type", "application/json")
	Head.Add("Access-Control-Allow-Origin", "*")
	Head.Add("Access-Control-Allow-Methods", "POST")

	req := r.URL.Query()
	req_auth := r.Header.Get("Authorization")

	if req_auth != AUTH {
		writeJSONError(w, "Invalid auth", http.StatusUnauthorized)
		return
	}

	var columns []string
	var placeholders []string
	var values []interface{}

	i := 1
	for key, val := range req {
		if key != "auth" {
			columns = append(columns, key)
			placeholders = append(placeholders, fmt.Sprintf("$%d", i))
			values = append(values, val[0])
			i++
		}
	}

	query := fmt.Sprintf("INSERT INTO %s (%s) VALUES (%s)",
		TABLE, strings.Join(columns, ","), strings.Join(placeholders, ","))

	result, err := db.Exec(query, values...)
	if err != nil {
		writeJSONError(w, "Error inserting data: "+err.Error(), http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		writeJSONError(w, "Error getting affected rows", http.StatusInternalServerError)
		return
	}

	response := fmt.Sprintf("{\"msg\": \"ok\", \"affected\": %d}", rowsAffected)
	w.Write([]byte(response))
}

func UpdateInvoice(w http.ResponseWriter, r *http.Request, AUTH string, TABLE string) {
	Head := w.Header()

	db, err := ConnectDB()
	if err != nil {
		writeJSONError(w, "Error connecting to database", http.StatusInternalServerError)
		return
	}
	defer db.Close()

	Head.Add("Content-Type", "application/json")
	Head.Add("Access-Control-Allow-Origin", "*")
	Head.Add("Access-Control-Allow-Methods", "PUT")

	req := r.URL.Query()
	req_auth := r.Header.Get("Authorization")

	if req_auth != AUTH {
		writeJSONError(w, "Invalid auth", http.StatusUnauthorized)
		return
	}

	req_id := req.Get("id")
	if req_id == "" {
		writeJSONError(w, "Missing 'id' parameter", http.StatusBadRequest)
		return
	}

	var updates []string
	var values []interface{}

	i := 1
	for key, val := range req {
		if key != "id" && key != "auth" {
			updates = append(updates, fmt.Sprintf("%s = $%d", key, i))
			values = append(values, val[0])
			i++
		}
	}

	query := fmt.Sprintf("UPDATE %s SET %s WHERE id = $%d",
		TABLE, strings.Join(updates, ","), i)
	values = append(values, req_id)

	result, err := db.Exec(query, values...)
	if err != nil {
		writeJSONError(w, "Error updating data", http.StatusInternalServerError)
		return
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		writeJSONError(w, "Error getting affected rows", http.StatusInternalServerError)
		return
	}

	response := fmt.Sprintf("{\"msg\": \"ok\", \"affected\": %d}", rowsAffected)
	w.Write([]byte(response))
}
