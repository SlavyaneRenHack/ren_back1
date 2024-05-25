package main

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"github.com/gomodule/redigo/redis"
	"database/sql"
	_ "github.com/lib/pq"
	"strings"
)

var poolShopsv = poolShops()
var poolPg = poolpg()

func generateChecksum(payID, shopID, amount, privateKey string) string {
	data := []byte(payID + "&" + shopID + "&" + amount + "&" + privateKey)
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash[:])
}

func handleTokenRequest(w http.ResponseWriter, r *http.Request) {
	if r.Method!= http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data struct {
		PayID   string `json:"payId"`
		ShopID  string `json:"shopId"`
		Amount  int    `json:"amount"`
		Checksum string `json:"checksum"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err!= nil {
		fmt.Fprint(w, "err3")
		return
	}

	result, err := poolPg.Query("select seckey from shops where shopid=$1", data.ShopID)
	if err != nil{
		panic(err)
	}
	var seckey string
	result.Next()
		err = result.Scan(&seckey)
		if err != nil{
			fmt.Println(err)
		}

	checksum := generateChecksum(data.PayID, data.ShopID, fmt.Sprint(data.Amount), seckey)
	if checksum==data.Checksum {
		client := poolShopsv.Get()
		defer client.Close()
		client.Do("HSET", checksum, "payid", data.PayID, "shopid", data.ShopID, "amount", data.Amount)
		client.Do("EXPIRE", checksum, 300)
	}
	// Здесь должна быть логика обработки запроса к внешнему сервису
	// Например, проверка checksum и отправка подтверждения обратно клиенту
	fmt.Fprintf(w, "Received token request for payId: %s, ShopID: %s, Amount: %d", data.PayID, data.ShopID, data.Amount)
}

func handleClientPay(w http.ResponseWriter, r *http.Request) {
	// Реализация эндпоинта /client_pay
	var data struct {
		Token string    `json:"token"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err!= nil {
		fmt.Fprint(w, "err2")
		//http.Error(w, r, http.StatusBadRequest)
		return
	}
	client := poolShopsv.Get()
	defer client.Close()
	tn1, _ := client.Do("KEYS", data.Token+"*")
	tn := fmt.Sprintf("%s",tn1)
	tn=strings.ReplaceAll(tn, "[","")
	tn=strings.ReplaceAll(tn, "]","")
	tn1, _ = client.Do("HMGET", tn, "shopid", "amount","payid")
	tn2 := fmt.Sprintf("%s",tn1)
	words := strings.Fields(tn2)
	words[0]=strings.ReplaceAll(words[0],"[","")
	words[2]=strings.ReplaceAll(words[2],"]","")
	result, err := poolPg.Query("select nameshop from shops where shopid=$1", words[0])
	if err != nil{
		panic(err)
	}
	var nameshop string
	result.Next()
	err = result.Scan(&nameshop)
	if err != nil{
		fmt.Println(err)
	}

	jsonData := map[string]interface{}{
		"payId":     tn,
		"amount":    words[1],
		"nameshop":    nameshop,
		}
	responseBody, _ := json.Marshal(jsonData)
	fmt.Fprintf(w, "%s", responseBody)
}

func handlePayStatus(w http.ResponseWriter, r *http.Request) {
	// Реализация эндпоинта /pay_status
	client := poolShopsv.Get()
	defer client.Close()
	var data struct {
		Payid string    `json:"payid"`
	}
	err := json.NewDecoder(r.Body).Decode(&data)
	if err!= nil {
		fmt.Fprint(w, "err2")
		//http.Error(w, r, http.StatusBadRequest)
		return
	}
	//fmt.Println(data.Payid)
	client.Do("DEL", data.Payid)
	fmt.Fprintln(w, "This is the /pay_status endpoint.")
}
func testhandler(w http.ResponseWriter, r *http.Request) {
	fmt.Fprint(w, "working")
}
func main() {
	http.HandleFunc("/token", handleTokenRequest)
	http.HandleFunc("/client_pay", handleClientPay)
	http.HandleFunc("/pay_status", handlePayStatus)
	http.HandleFunc("/test", testhandler)
	fmt.Println("Server is running on port 80")
	if err := http.ListenAndServe(":80", nil); err!= nil {
		panic(err)
	}
}
func poolShops() *redis.Pool {
	return &redis.Pool{
		MaxIdle: 80,
		MaxActive: 12000,
		Dial: func() (redis.Conn, error) {
			c, err := redis.Dial("tcp", "92.246.76.93:6379", redis.DialPassword("8?~9z5@1p2VM~:"))
			if err != nil {
				panic(err.Error())
			}
			return c, err
		},
	}
}
func poolpg() *sql.DB {
	connStr := "user=gen_user password=8?~9z5@1p2VM~: dbname=default_db sslmode=disable host=92.246.76.119"
	db, err := sql.Open("postgres", connStr)
	if err != nil {
		panic(err)
	}
	return db
}
