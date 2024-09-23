package main

import (
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/0x00f00bar/web-crawler/models"
	"github.com/0x00f00bar/web-crawler/models/psql"
)

// const baseURL = "https://bankofbaroda.in"
const prodURL = "https://www.bankofbaroda.in/personal-banking/accounts/saving-accounts/bob-lite-savings-account"

func main() {
	fmt.Println("Fetch all the pages!")

	client := &http.Client{
		Timeout: 5 * time.Second,
	}

	resp, err := getURL(prodURL, client)
	if err != nil {
		log.Fatalln(err)
	}

	defer resp.Body.Close()
	// body, err := io.ReadAll(resp.Body)
	// if err != nil {
	// 	log.Fatalln(err)
	// }
	for k, v := range resp.Header {
		fmt.Println(k, v)
	}
	// fmt.Println(string(body))
	var models models.Models
	var ddd *sql.DB

	db := psql.NewPsqlDB(ddd)
	models.Pages = db.PageModel
}
