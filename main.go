package main

import (
	"database/sql"
	"log"
	"net/http"
	"os"
	"strconv"
	//"unicode/utf8"

	"github.com/gin-gonic/contrib/sessions"
	//"github.com/gin-gonic/contrib/static"
	"github.com/gin-gonic/gin"
	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/redis.v3"
)

var db *sql.DB
var red *redis.Client

func main() {
	// database setting
	user := os.Getenv("ISHOCON1_DB_USER")
	pass := os.Getenv("ISHOCON1_DB_PASSWORD")
	dbname := "ishocon1"
	db, _ = sql.Open("mysql", user+":"+pass+"@unix(/var/lib/mysql/mysql.sock)/"+dbname)
	db.SetMaxIdleConns(10)

	red := redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "",
		DB:       0,
	})

	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	// load templates
	r.LoadHTMLGlob("templates/*")

	// session store
	store := sessions.NewCookieStore([]byte("showwin_happy"))
	store.Options(sessions.Options{HttpOnly: true})
	r.Use(sessions.Sessions("mysession", store))

	// GET /login
	r.GET("/login", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()

		c.HTML(http.StatusOK, "login", gin.H{
			"Message": "ECサイトで爆買いしよう！！！！",
		})
	})

	// POST /login
	r.POST("/login", func(c *gin.Context) {
		email := c.PostForm("email")
		pass := c.PostForm("password")

		session := sessions.Default(c)
		user, result := authenticate(email, pass)
		if result {
			// 認証成功
			session.Set("uid", user.ID)
			session.Save()

			//user.UpdateLastLogin()

			c.Redirect(http.StatusSeeOther, "/")
		} else {
			// 認証失敗
			c.HTML(http.StatusOK, "login", gin.H{
				"Message": "ログインに失敗しました",
			})
		}
	})

	// GET /logout
	r.GET("/logout", func(c *gin.Context) {
		session := sessions.Default(c)
		session.Clear()
		session.Save()

		c.HTML(http.StatusOK, "login", gin.H{
			"Message": "ECサイトで爆買いしよう！！！！",
		})
	})

	// GET /
	r.GET("/", func(c *gin.Context) {
		cUser := currentUserRedis(sessions.Default(c))

		page, err := strconv.Atoi(c.Query("page"))
		if err != nil {
			page = 0
		}
		products := getProductsWithCommentsAt(page)
		// shorten description and comment
		//var sProducts []ProductWithComments
		//for _, p := range products {
		//	if utf8.RuneCountInString(p.Description) > 70 {
		//		p.Description = string([]rune(p.Description)[:70]) + "…"
		//	}

		//	var newCW []CommentWriter
		//	for _, c := range p.Comments {
		//		if utf8.RuneCountInString(c.Content) > 25 {
		//			c.Content = string([]rune(c.Content)[:25]) + "…"
		//		}
		//		newCW = append(newCW, c)
		//	}
		//	p.Comments = newCW
		//	sProducts = append(sProducts, p)
		//}

		c.HTML(http.StatusOK, "index", gin.H{
			"CurrentUser": cUser,
			//"Products":    sProducts,
			"Products": products,
		})
	})

	// GET /users/:userId
	r.GET("/users/:userId", func(c *gin.Context) {
		cUser := currentUserRedis(sessions.Default(c))

		uid, _ := strconv.Atoi(c.Param("userId"))
		user := getUser(uid)

		products := user.BuyingHistory()

		totalPay, _ := red.Get(c.Param("userId") + "s").Result()
		//for _, p := range products {
		//	totalPay += p.Price
		//}

		// shorten description
		//var sdProducts []Product
		//for _, p := range products {
		//	if utf8.RuneCountInString(p.Description) > 70 {
		//		p.Description = string([]rune(p.Description)[:70]) + "…"
		//	}
		//	sdProducts = append(sdProducts, p)
		//}

		c.HTML(http.StatusOK, "mypage", gin.H{
			"CurrentUser": cUser,
			"User":        user,
			//"Products":    sdProducts,
			"Products": products,
			"TotalPay": totalPay,
		})
	})

	// GET /products/:productId
	r.GET("/products/:productId", func(c *gin.Context) {
		pid, _ := strconv.Atoi(c.Param("productId"))
		product := getProduct(pid)
		//comments := getComments(pid)

		cUser := currentUserRedis(sessions.Default(c))
		bought := product.isBought(cUser.ID)

		c.HTML(http.StatusOK, "product", gin.H{
			"CurrentUser": cUser,
			"Product":     product,
			//"Comments":      comments,
			"AlreadyBought": bought,
		})
	})

	// POST /products/buy/:productId
	r.POST("/products/buy/:productId", func(c *gin.Context) {
		// need authenticated
		if notAuthenticated(sessions.Default(c)) {
			c.HTML(http.StatusForbidden, "login", gin.H{
				"Message": "先にログインをしてください",
			})
		} else {
			// buy product
			cUser := currentUserRedis(sessions.Default(c))
			cUser.BuyProduct(c.Param("productId"))

			// sum
			price, _ := red.Get(c.Param("productId") + "p").Result()
			price_str, _ := strconv.ParseInt(price, 10, 16)
			red.IncrBy(strconv.Itoa(cUser.ID)+"s", price_str)

			// add parchased list
			red.LPush(strconv.Itoa(cUser.ID)+"l", c.Param("productId"))

			// redirect to user page
			c.Redirect(http.StatusFound, "/users/"+strconv.Itoa(cUser.ID))
		}
	})

	// POST /comments/:productId
	r.POST("/comments/:productId", func(c *gin.Context) {
		// need authenticated
		if notAuthenticated(sessions.Default(c)) {
			c.HTML(http.StatusForbidden, "login", gin.H{
				"Message": "先にログインをしてください",
			})
		} else {
			// create comment
			cUser := currentUserRedis(sessions.Default(c))
			//cUser.CreateComment(c.Param("productId"), c.PostForm("content"))
			red.Incr(c.Param("productId") + "c")

			// redirect to user page
			c.Redirect(http.StatusFound, "/users/"+strconv.Itoa(cUser.ID))
		}
	})

	// GET /initialize
	r.GET("/initialize", func(c *gin.Context) {
		db.Exec("DELETE FROM users WHERE id > 5000")
		db.Exec("DELETE FROM products WHERE id > 10000")
		db.Exec("DELETE FROM comments WHERE id > 200000")
		db.Exec("DELETE FROM histories WHERE id > 500000")
		db.Exec("DELETE FROM commentsuser WHERE id > 200000")

		// initialize redis
		red.FlushAll()

		// number of comment
		for i := 1; i <= 10000; i++ {
			red.Set(strconv.Itoa(i)+"c", 20, 0)
		}

		// sum of purchased price
		for i := 1; i <= 5000; i++ {
			var sum int
			db.QueryRow("SELECT sum FROM sums WHERE user_id = ?", i).Scan(&sum)
			red.Set(strconv.Itoa(i)+"s", sum, 0)
		}

		// key: product_id, value: price
		for i := 1; i <= 10000; i++ {
			var price int
			db.QueryRow("SELECT price from products WHERE id = ?", i).Scan(&price)
			red.Set(strconv.Itoa(i)+"p", price, 0)
		}

		// recent 30 purchased list
		for i := 1; i <= 5000; i++ {
			rows, _ := db.Query("SELECT product_id from histories WHERE user_id = ? ORDER BY id DESC LIMIT 30", i)
			defer rows.Close()
			for rows.Next() {
				var pid int
				rows.Scan(&pid)
				err := red.RPush(strconv.Itoa(i)+"l", strconv.Itoa(pid)).Err()
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		// user info
		rows, _ := db.Query("SELECT id, name from users")
		defer rows.Close()
		for rows.Next() {
			var id int
			var name string
			rows.Scan(&id, &name)
			red.Set(strconv.Itoa(id)+"u", name, 0)
		}

		c.String(http.StatusOK, "Finish")
	})

	r.Run(":8080")
}
