package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

func generateToken() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

var db *sql.DB

func log(name string) {
	fmt.Println("Совершен вход в ", name)
}

func InitDB() {
	var err error
	db, err = sql.Open("sqlite", "./users.db")
	if err != nil {
		panic(err)
	}

	db.Exec(`CREATE TABLE IF NOT EXISTS users (
        id INTEGER PRIMARY KEY AUTOINCREMENT,
        name TEXT UNIQUE,
        password TEXT,
        balance INTEGER DEFAULT 0,
        token TEXT UNIQUE
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

func GetUserByToken(token string) (string, error) {
	var name string
	err := db.QueryRow(`SELECT name FROM users WHERE token = ?`, token).Scan(&name)
	return name, err
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
		fmt.Printf("Недостаточно средств! Баланс: %d, требуется: %d\n", currentBalance, totalDeduction)
		return currentBalance
	}

	db.Exec(`UPDATE users SET balance = balance - ? WHERE name = ?`, totalDeduction, name)

	fmt.Printf("%s вывел %d, спишется %d\n", name, withdraw, totalDeduction)

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

		token := generateToken()
		db.Exec(`UPDATE users SET token = ? WHERE name = ?`, token, name)

		cookie := &http.Cookie{
			Name:     "token",
			Value:    token,
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
			token := generateToken()
			db.Exec(`UPDATE users SET token = ? WHERE name = ?`, token, name)

			cookie := &http.Cookie{
				Name:     "token",
				Value:    token,
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

	http.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		cookie, err := r.Cookie("token")
		if err == nil {
			db.Exec(`UPDATE users SET token = NULL WHERE token = ?`, cookie.Value)
		}

		http.SetCookie(w, &http.Cookie{
			Name:     "token",
			Value:    "",
			Expires:  time.Now().Add(-1 * time.Hour),
			HttpOnly: true,
			Path:     "/",
		})

		http.Redirect(w, r, "/", http.StatusFound)
	})

	http.HandleFunc("/transfer", func(w http.ResponseWriter, r *http.Request) {
		log("перевод")
		name1 := r.URL.Query().Get("name1")
		sumStr := r.URL.Query().Get("sum")

		sum, err := strconv.Atoi(sumStr)
		if err != nil {
			fmt.Fprint(w, "Сумма должна быть числом")
			return
		}

		cookie, err := r.Cookie("token")
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		name, err := GetUserByToken(cookie.Value)
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		if name == name1 {
			fmt.Fprint(w, "Нельзя переводить самому себе")
			return
		}

		if GetUserName(name1) == "" {
			fmt.Fprint(w, "Получатель не найден")
			return
		}

		if GetBalance(name) < sum {
			fmt.Fprint(w, "Недостаточно средств")
			return
		}

		BankWithdraw(name, sum)
		BankDeposit(name1, sum)

		fmt.Fprint(w, "Перевод выполнен")
	})

	http.HandleFunc("/deposit", func(w http.ResponseWriter, r *http.Request) {
		log("депозит")
		depositStr := r.URL.Query().Get("deposit")

		deposit, err := strconv.Atoi(depositStr)
		if err != nil {
			fmt.Fprint(w, "Сумма должна быть числом")
			return
		}

		cookie, err := r.Cookie("token")
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		name, err := GetUserByToken(cookie.Value)
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		result := BankDeposit(name, deposit)
		fmt.Fprint(w, result)
	})

	http.HandleFunc("/withdraw", func(w http.ResponseWriter, r *http.Request) {
		log("withdraw")
		withdrawStr := r.URL.Query().Get("deposit")

		withdraw, err := strconv.Atoi(withdrawStr)
		if err != nil {
			fmt.Fprint(w, "Сумма должна быть числом")
			return
		}

		cookie, err := r.Cookie("token")
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		name, err := GetUserByToken(cookie.Value)
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		result := BankWithdraw(name, withdraw)
		fmt.Fprint(w, result)
	})

	http.HandleFunc("/get_balance", func(w http.ResponseWriter, r *http.Request) {
		log("баланс")

		cookie, err := r.Cookie("token")
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		name, err := GetUserByToken(cookie.Value)
		if err != nil {
			fmt.Fprint(w, "Не авторизован")
			return
		}

		balance := GetBalance(name)
		fmt.Fprintf(w, "Баланс: %d", balance)
	})

	http.HandleFunc("/register-page", func(w http.ResponseWriter, r *http.Request) {
		log("страницу регистрации")
		tmpl := template.Must(template.ParseFiles("./static/register.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/login-page", func(w http.ResponseWriter, r *http.Request) {
		log("страницу входа")
		tmpl := template.Must(template.ParseFiles("./static/login.html"))
		tmpl.Execute(w, nil)
	})

	http.HandleFunc("/profile", func(w http.ResponseWriter, r *http.Request) {
		log("профиль")

		cookie, err := r.Cookie("token")
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusFound)
			return
		}

		name, err := GetUserByToken(cookie.Value)
		if err != nil {
			http.Redirect(w, r, "/login-page", http.StatusFound)
			return
		}

		var balance int
		err = db.QueryRow(`SELECT balance FROM users WHERE name = ?`, name).Scan(&balance)
		if err != nil {
			balance = 0
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
