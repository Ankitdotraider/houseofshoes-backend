package handlers

import (
	"database/sql"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/Ankitdotraider/houseofshoes/internal/db"
	"github.com/Ankitdotraider/houseofshoes/internal/models"
	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// ─── Auth ───────────────────────────────────────────────────────────────────

func Signup(c *gin.Context) {
	var req models.SignupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), 12)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not hash password"})
		return
	}

	var id int
	err = db.DB.QueryRow(
		`INSERT INTO users (email, password) VALUES ($1, $2) RETURNING id`,
		req.Email, string(hash),
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "email already exists"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id, "email": req.Email})
}

func Login(c *gin.Context) {
	var req models.LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var user models.User
	err := db.DB.QueryRow(
		`SELECT id, email, password, role FROM users WHERE email=$1`, req.Email,
	).Scan(&user.ID, &user.Email, &user.Password, &user.Role)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.Password), []byte(req.Password)); err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
	})

	tokenStr, err := token.SignedString([]byte(os.Getenv("JWT_SECRET")))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not generate token"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"token": tokenStr, "role": user.Role})
}

// ─── Products ────────────────────────────────────────────────────────────────

func GetProducts(c *gin.Context) {
	category := c.Query("category")
	brand := c.Query("brand")

	query := `SELECT id, name, brand, category, gender, occasion, description, price, image_url FROM products WHERE 1=1`
	args := []interface{}{}
	i := 1

	if category != "" {
		query += ` AND category=$` + strconv.Itoa(i)
		args = append(args, category)
		i++
	}
	if brand != "" {
		query += ` AND brand=$` + strconv.Itoa(i)
		args = append(args, brand)
		i++
	}

	rows, err := db.DB.Query(query, args...)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	products := []models.Product{}
	for rows.Next() {
		var p models.Product
		rows.Scan(&p.ID, &p.Name, &p.Brand, &p.Category, &p.Gender, &p.Occasion, &p.Description, &p.Price, &p.ImageURL)
		products = append(products, p)
	}

	c.JSON(http.StatusOK, products)
}

func GetProduct(c *gin.Context) {
	id := c.Param("id")

	var p models.Product
	err := db.DB.QueryRow(
		`SELECT id, name, brand, category, description, image_url FROM products WHERE id=$1`, id,
	).Scan(&p.ID, &p.Name, &p.Brand, &p.Category, &p.Description, &p.ImageURL)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	rows, err := db.DB.Query(`SELECT id, size, price, stock FROM product_variants WHERE product_id=$1`, p.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	variants := []models.ProductVariant{}
	for rows.Next() {
		var v models.ProductVariant
		rows.Scan(&v.ID, &v.Size, &v.Price, &v.Stock)
		variants = append(variants, v)
	}

	c.JSON(http.StatusOK, gin.H{"product": p, "variants": variants})
}

func CreateProduct(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	var p models.Product
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id int
	err := db.DB.QueryRow(
    `INSERT INTO products (name, brand, category, gender, occasion, description, image_url, price) VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id`,
p.Name, p.Brand, p.Category, p.Gender, p.Occasion, p.Description, p.ImageURL, p.Price,
).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateProduct(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	id := c.Param("id")
	var p models.Product
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	_, err := db.DB.Exec(
		`UPDATE products SET name=$1, brand=$2, category=$3, gender=$4, occasion=$5, description=$6, image_url=$7 WHERE id=$8`,
p.Name, p.Brand, p.Category, p.Gender, p.Occasion, p.Description, p.ImageURL, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func DeleteProduct(c *gin.Context) {
	role, _ := c.Get("role")
	if role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	id := c.Param("id")
	_, err := db.DB.Exec(`DELETE FROM products WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ─── Orders ──────────────────────────────────────────────────────────────────

func CreateOrder(c *gin.Context) {
	userID, _ := c.Get("user_id")

	var order models.Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var orderID int
	err := db.DB.QueryRow(
		`INSERT INTO orders (user_id, total, address) VALUES ($1,$2,$3) RETURNING id`,
		userID, order.Total, order.Address,
	).Scan(&orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	for _, item := range order.Items {
		_, err := db.DB.Exec(
			`INSERT INTO order_items (order_id, variant_id, quantity, price) VALUES ($1,$2,$3,$4)`,
			orderID, item.VariantID, item.Quantity, item.Price,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		db.DB.Exec(`UPDATE product_variants SET stock=stock-$1 WHERE id=$2`, item.Quantity, item.VariantID)
	}

	c.JSON(http.StatusCreated, gin.H{"order_id": orderID})
}

func GetOrders(c *gin.Context) {
	userID, _ := c.Get("user_id")
	role, _ := c.Get("role")

	var rows *sql.Rows
	var err error

	if role == "admin" {
		rows, err = db.DB.Query(`SELECT id, user_id, status, total, address, created_at FROM orders ORDER BY created_at DESC`)
	} else {
		rows, err = db.DB.Query(`SELECT id, user_id, status, total, address, created_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	orders := []models.Order{}
	for rows.Next() {
		var o models.Order
		rows.Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.Address, &o.CreatedAt)
		orders = append(orders, o)
	}

	c.JSON(http.StatusOK, orders)
}