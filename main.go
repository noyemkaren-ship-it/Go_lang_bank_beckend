package main

import (
	"database/sql"
	"fmt"
	"html/template"
	"net/http"
	"strconv"

	_ "modernc.org/sqlite"
)

var db *sql.DB

func log(name string) {
	fmt.Println("Совершен вход в ", name)
}

func InitDB() {
	var err error
	db, err = sql.Open("sqlite", "./test.db")
	if err != nil {
		panic(err)
	}

	db.Exec(`CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT UNIQUE,
        password TEXT,
        balance INTEGER DEFAULT 0
    )`)
}

func UserExists(name string) bool {
	var id int
	err := db.QueryRow(`SELECT id FROM users WHERE name = ?`, name).Scan(&id)
	return err == nil
}

func CreateUser(name, password string) error {
	if name == "" || password == "" {
		return fmt.Errorf("имя и пароль обязательны")
	}

	if UserExists(name) {
		return fmt.Errorf("пользователь уже существует")
	}

	_, err := db.Exec(`INSERT INTO users (name, password) VALUES (?, ?)`, name, password)
	return err
}

func Login(name, password string) bool {
	var id int
	err := db.QueryRow(`SELECT id FROM users WHERE name = ? AND password = ?`, name, password).Scan(&id)
	return err == nil
}

func GetUserName(name string) string {
	var dbName string
	db.QueryRow(`SELECT name FROM users WHERE name = ?`, name).Scan(&dbName)
	return dbName
}

func RegisterFunction(name, password string) error {
	if UserExists(name) {
		return fmt.Errorf("пользователь '%s' уже существует", name)
	}

	if name == "" || password == "" {
		return fmt.Errorf("имя и пароль не могут быть пустыми")
	}

	return CreateUser(name, password)
}

func BankDeposit(name string, deposit int) int {
	if deposit < 0 {
		fmt.Println("Сумма депозита не может быть отрицательной")
		return GetBalance(name)
	}

	if deposit > 10000 {
		commission := deposit * 5 / 100
		deposit -= commission
		fmt.Printf("Комиссия 5%%: %d\n", commission)
	}

	db.Exec(`UPDATE users SET balance = balance + ? WHERE name = ?`, deposit, name)

	fmt.Printf("%s внёс %d\n", name, deposit)

	return GetBalance(name)
}

func BankWithdraw(name string, withdraw int) int {
	if withdraw <= 0 {
		fmt.Println("Сумма вывода должна быть положительной")
		return GetBalance(name)
	}

	currentBalance := GetBalance(name)

	totalDeduction := withdraw
	if withdraw > 10000 {
		commission := withdraw * 5 / 100
		totalDeduction = withdraw + commission
		fmt.Printf("Комиссия 5%%: %d, всего спишется: %d\n", commission, totalDeduction)
	}

	if currentBalance < totalDeduction {
		fmt.Printf("❌ Недостаточно средств! Баланс: %d, требуется: %d\n", currentBalance, totalDeduction)
		return currentBalance // Возвращаем текущий баланс без изменений
	}

	db.Exec(`UPDATE users SET balance = balance - ? WHERE name = ?`, totalDeduction, name)

	fmt.Printf("✅ %s вывел %d, спишется %d\n", name, withdraw, totalDeduction)

	return GetBalance(name)
}

func GetBalance(name string) int {
	var balance int
	db.QueryRow(`SELECT balance FROM users WHERE name = ?`, name).Scan(&balance)
	return balance
}

func main() {
	InitDB()
	http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.Dir("./static"))))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		http.ServeFile(w, r, "./static/index.html")
		log("главную страницу")
	})

	http.HandleFunc("/register", func(w http.ResponseWriter, r *http.Request) {
		log("регистрацию")
		name := r.URL.Query().Get("name")
		password := r.URL.Query().Get("password")

		err := RegisterFunction(name, password)
		if err != nil {
			fmt.Fprintf(w, "Ошибка: %v", err)
			return
		}

		cookie := &http.Cookie{
			Name:     "token",
			Value:    name,
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			Path:     "/",
		}
		http.SetCookie(w, cookie)
		fmt.Fprintf(w, "Пользователь %s зарегистрирован", name)
	})

	http.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		log("логин")
		name := r.URL.Query().Get("name")
		password := r.URL.Query().Get("password")
		if Login(name, password) {
			cookie := &http.Cookie{
				Name:     "token",
				Value:    name,
				HttpOnly: true,
				SameSite: http.SameSiteLaxMode,
				Path:     "/",
			}
			http.SetCookie(w, cookie)
			fmt.Fprint(w, "Вы вошли в систему")
			return
		}
		fmt.Fprint(w, "Не удалось войти")
	})

	http.HandleFunc("/deposit", func(w http.ResponseWriter, r *http.Request) {
		log("депозит")
		name := r.URL.Query().Get("name")
		depositStr := r.URL.Query().Get("deposit")

		deposit, err := strconv.Atoi(depositStr)
		if err != nil {
			fmt.Fprint(w, "Сумма должна быть числом")
			return
		}

		result := BankDeposit(name, deposit)
		fmt.Fprint(w, result)
	})

	http.HandleFunc("/withdraw", func(w http.ResponseWriter, r *http.Request) {
		log("withdraw")
		name := r.URL.Query().Get("name")
		withdrawStr := r.URL.Query().Get("deposit")

		deposit, err := strconv.Atoi(withdrawStr)
		if err != nil {
			fmt.Fprint(w, "Сумма должна быть числом")
			return
		}

		result := BankWithdraw(name, deposit)
		fmt.Fprint(w, result)
	})

	http.HandleFunc("/get_balance", func(w http.ResponseWriter, r *http.Request) {
		log("баланс")
		name := r.URL.Query().Get("name")

		cookie, err := r.Cookie("token")
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		if cookie.Value != name {
			fmt.Fprint(w, "Доступ запрещён")
			return
		}

		balance := GetBalance(name)
		fmt.Fprintf(w, "Баланс: %d", balance)
	})

	http.HandleFunc("/register-page", func(w http.ResponseWriter, r *http.Request) {
		log("регистрацию")
		tmpl := template.Must(template.ParseFiles("./static/register.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/login-page", func(w http.ResponseWriter, r *http.Request) {
		log("логин")
		tmpl := template.Must(template.ParseFiles("./static/login.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		log("профиль")

		cookie, err := r.Cookie("token")
		if err != nil {
			http.Redirect(w, r, "/login", http.StatusFound)
			return
		}

		name := cookie.Value

		var balance int
		err = db.QueryRow(`SELECT balance FROM users WHERE name = ?`, name).Scan(&balance)
		if err != nil {
			balance = 0
		}

		if name == "" {
			tmpl := template.Must(template.ParseFiles("./static/error.html"))
			tmpl.Execute(w, nil)
			return
		}

		data := struct {
			Name    string
			Balance int
		}{
			Name:    name,
			Balance: balance,
		}

		tmpl := template.Must(template.ParseFiles("./static/profile.html"))
		tmpl.Execute(w, data)
	})

	fmt.Println("Сервер запущен на http://localhost:8080")
	http.ListenAndServe(":8080", nil)
}
