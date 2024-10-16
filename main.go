package main

import (
	"encoding/json"
	"log"
	"net/http"
	"strconv"

	"github.com/go-pg/pg/v10"
	"github.com/go-pg/pg/v10/orm"
	"github.com/go-playground/validator/v10"
	"github.com/gorilla/mux"
)

// User структура для хранения информации о пользователе
type User struct {
	ID    int    `json:"id"`
	Name  string `json:"name" validate:"required,min=2,max=100"`
	Email string `json:"email" validate:"required,email"`
	Age   int    `json:"age" validate:"gte=0,lte=130"`
}

// AuthRequest структура для хранения данных авторизации
type AuthRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

var db *pg.DB
var validate *validator.Validate

// connectDB функция для подключения к базе данных
func connectDB() *pg.DB {
	opt, err := pg.ParseURL("postgres://admin:admin@localhost:5432/mydb?sslmode=disable")
	if err != nil {
		log.Fatalf("Error connecting to database: %v", err)
	}
	db := pg.Connect(opt)
	if db == nil {
		log.Fatalf("Failed to connect to the database.")
	}
	log.Println("Connection to the database successful.")
	return db
}

// createSchema функция для создания таблицы в базе данных
func createSchema() error {
	err := db.Model((*User)(nil)).CreateTable(&orm.CreateTableOptions{
		IfNotExists: true,
	})
	return err
}

// init функция для инициализации базы данных и валидатора
func init() {
	db = connectDB()
	validate = validator.New()

	// Создание таблицы для пользователей
	err := createSchema()
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}
}

// getUsers функция для получения списка пользователей с поддержкой пагинации и фильтрации
func getUsers(w http.ResponseWriter, r *http.Request) {
	pageStr := r.URL.Query().Get("page")
	limitStr := r.URL.Query().Get("limit")
	name := r.URL.Query().Get("name")
	ageStr := r.URL.Query().Get("age")

	page, err := strconv.Atoi(pageStr)
	if err != nil {
		page = 1
	}
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	// Фильтрация по имени и возрасту
	var users []User
	query := db.Model(&users)
	if name != "" {
		query = query.Where("name = ?", name)
	}
	if ageStr != "" {
		age, _ := strconv.Atoi(ageStr)
		query = query.Where("age = ?", age)
	}

	// Пагинация
	err = query.Offset((page - 1) * limit).Limit(limit).Select()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(users)
}

// getUser функция для получения конкретного пользователя по ID
func getUser(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])

	user := &User{ID: id}
	err := db.Model(user).WherePK().Select()
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(user)
}

// createUser функция для создания нового пользователя
func createUser(w http.ResponseWriter, r *http.Request) {
	var user User
	_ = json.NewDecoder(r.Body).Decode(&user)

	// Валидация данных
	if err := validate.Struct(user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	// Сохранение в базу данных
	_, err := db.Model(&user).Insert()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(user)
}

// updateUser функция для обновления информации о пользователе
func updateUser(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])

	var user User
	_ = json.NewDecoder(r.Body).Decode(&user)

	// Валидация данных
	if err := validate.Struct(user); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	user.ID = id
	_, err := db.Model(&user).Where("id = ?", id).Update()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(user)
}

// deleteUser функция для удаления пользователя
func deleteUser(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	id, _ := strconv.Atoi(params["id"])

	user := &User{ID: id}
	_, err := db.Model(user).WherePK().Delete()
	if err != nil {
		http.Error(w, "User not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"message": "User deleted"})
}

// loginHandler функция для обработки авторизации
func loginHandler(w http.ResponseWriter, r *http.Request) {
	var authReq AuthRequest
	err := json.NewDecoder(r.Body).Decode(&authReq)
	if err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	log.Printf("Received username: %s, password: %s", authReq.Username, authReq.Password)

	if authReq.Username == "user" && authReq.Password == "password" {
		token := map[string]string{"token": "your_token_here"}
		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(token)
	} else {
		log.Println("Unauthorized attempt")
		http.Error(w, "Unauthorized", http.StatusUnauthorized)
	}
}

func main() {
	router := mux.NewRouter()

	// Маршруты
	router.HandleFunc("/login", loginHandler).Methods("POST")
	router.HandleFunc("/users", getUsers).Methods("GET")
	router.HandleFunc("/users/{id}", getUser).Methods("GET")
	router.HandleFunc("/users", createUser).Methods("POST")
	router.HandleFunc("/users/{id}", updateUser).Methods("PUT")
	router.HandleFunc("/users/{id}", deleteUser).Methods("DELETE")

	log.Println("Server started at :8000")
	log.Fatal(http.ListenAndServe(":8000", router))
}

// curl -X GET http://localhost:8000/users

// curl -X GET http://localhost:8000/users/1

// curl -X POST http://localhost:8000/users -H "Content-Type: application/json" -d '{"name": "John Doe", "email": "johndoe@example.com", "age": 30}'

// curl -X PUT http://localhost:8000/users/1 -H "Content-Type: application/json" -d '{"name": "Jane Doe", "email": "janedoe@example.com", "age": 25}'

// curl -X DELETE http://localhost:8000/users/1

// С пагинацией и фильтр лимит=5 curl -X GET "http://localhost:8000/users?page=2&limit=5&name=John"

// TRUNCATE TABLE users RESTART IDENTITY;
