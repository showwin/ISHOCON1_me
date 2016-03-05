package main

import (
	"gopkg.in/redis.v3"
	"log"
	"strconv"
)

// Product Model
type Product struct {
	ID          int
	Name        string
	Description string
	ImagePath   string
	Price       int
	CreatedAt   string
}

// ProductWithComments Model
type ProductWithComments struct {
	ID           int
	Name         string
	Description  string
	ImagePath    string
	Price        int
	CreatedAt    string
	CommentCount string
	Comments     []CommentWriter
}

// CommentWriter Model
type CommentWriter struct {
	Content string
	Writer  string
}

var cli2 = redis.NewClient(&redis.Options{
	Addr:     "localhost:6379",
	Password: "",
	DB:       0,
})

func getProduct(pid int) Product {
	p := Product{}
	row := db.QueryRow("SELECT * FROM products WHERE id = " + strconv.Itoa(pid) + " LIMIT 1")
	err := row.Scan(&p.ID, &p.Name, &p.Description, &p.ImagePath, &p.Price, &p.CreatedAt)
	if err != nil {
		panic(err.Error())
	}

	return p
}

func getProductsWithCommentsAt(page int) []ProductWithComments {
	// select 50 products with offset page*50
	products := []ProductWithComments{}
	from := 10000 - (page+1)*50
	to := 10000 - page*50
	rows, err := db.Query("SELECT * FROM products_short WHERE id > " + strconv.Itoa(from) + " and id <= " + strconv.Itoa(to) + " ORDER BY ID DESC")
	if err != nil {
		return nil
	}

	defer rows.Close()
	for rows.Next() {
		p := ProductWithComments{}
		err = rows.Scan(&p.ID, &p.Name, &p.Description, &p.ImagePath, &p.Price, &p.CreatedAt)

		// select comment count for the product
		//var cnt int
		//cnterr := db.QueryRow("SELECT count(*) as count FROM comments WHERE product_id = ?", p.ID).Scan(&cnt)
		//if cnterr != nil {
		//	cnt = 0
		//}
		//p.CommentCount = cnt

		p.CommentCount, _ = cli2.Get(strconv.Itoa(p.ID) + "c").Result()
		//if err != nil {
		//	log.Fatal(err)
		//}

		//if cnt > 0 {
		// select 5 comments and its writer for the product
		//var cWriters []CommentWriter

		//subrows, suberr := db.Query("SELECT content, name FROM commentsuser WHERE product_id = " + strconv.Itoa(p.ID))
		//subrows, suberr := db.Query("SELECT * FROM comments as c INNER JOIN users as u "+
		//	"ON c.user_id = u.id WHERE c.product_id = ? ORDER BY c.id DESC LIMIT 5", p.ID)
		//if suberr != nil {
		//	subrows = nil
		//}

		//defer subrows.Close()
		//for subrows.Next() {
		//	var cw CommentWriter
		//	subrows.Scan(&cw.Content, &cw.Writer)
		//	cWriters = append(cWriters, cw)
		//}

		//p.Comments = cWriters
		//}

		products = append(products, p)
	}

	return products
}

func (p *Product) isBought(uid int) bool {
	var count int
	log.Print(uid)
	log.Print(p.ID)
	err := db.QueryRow(
		"SELECT id as count FROM histories WHERE product_id = " + strconv.Itoa(p.ID) + " AND user_id = " + strconv.Itoa(uid) + " LIMIT 1").Scan(&count)
	if err != nil {
		return false
	}

	return count > 0
}
