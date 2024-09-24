package main

import (
	"fmt"

	"github.com/0x00f00bar/web-crawler/models"
)

// const baseURL = "https://bankofbaroda.in"
const prodURL = "https://www.bankofbaroda.in/personal-banking/accounts/saving-accounts/bob-lite-savings-account"

func main() {
	fmt.Println("Fetch all the pages!")

	// client := &http.Client{
	// 	Timeout: 5 * time.Second,
	// }

	// resp, err := getURL(prodURL, client)
	// if err != nil {
	// 	log.Fatalln(err)
	// }

	// defer resp.Body.Close()
	// // body, err := io.ReadAll(resp.Body)
	// // if err != nil {
	// // 	log.Fatalln(err)
	// // }
	// for k, v := range resp.Header {
	// 	fmt.Println(k, v)
	// }

	fmt.Println(models.ValidOrderBy("id", []string{"id", "created_at", "url_id", "content"}))
	fmt.Println(models.ValidOrderBy("url", []string{"id", "created_at", "url_id", "content"}))
	fmt.Println(models.ValidOrderBy("url_id", []string{"id", "created_at", "url_id", "content"}))
	fmt.Println(models.ValidOrderBy("created", []string{"id", "created_at", "url_id", "content"}))
	fmt.Println(models.ValidOrderBy("i", []string{"id", "created_at", "url_id", "content"}))
	fmt.Println(models.ValidOrderBy("lol", []string{"id", "created_at", "url_id", "content"}))

	// fmt.Println(string(body))
	// var models models.Models
	// var ddd *sql.DB

	// db := psql.NewPsqlDB(ddd)
	// models.Pages = db.PageModel
}
