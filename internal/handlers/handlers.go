package handlers

import (
	"bytes"
	"crypto/hmac"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
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

	secret := os.Getenv("JWT_SECRET")
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server configuration error: JWT_SECRET not set"})
		return
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"user_id": user.ID,
		"role":    user.Role,
		"exp":     time.Now().Add(7 * 24 * time.Hour).Unix(),
	})

	tokenStr, err := token.SignedString([]byte(secret))
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
	gender := c.Query("gender")
	occasion := c.Query("occasion")

	query := `SELECT id, name, COALESCE(brand,''), COALESCE(category,''), COALESCE(gender,''), COALESCE(occasion,''), COALESCE(description,''), COALESCE(image_url,''), COALESCE(price,0) FROM products WHERE 1=1`
	args := []interface{}{}
	i := 1

	if category != "" {
		query += ` AND LOWER(category)=$` + strconv.Itoa(i)
		args = append(args, strings.ToLower(category))
		i++
	}
	if brand != "" {
		query += ` AND LOWER(brand)=$` + strconv.Itoa(i)
		args = append(args, strings.ToLower(brand))
		i++
	}
	if gender != "" {
		query += ` AND LOWER(gender)=$` + strconv.Itoa(i)
		args = append(args, strings.ToLower(gender))
		i++
	}
	if occasion != "" {
		query += ` AND LOWER(occasion)=$` + strconv.Itoa(i)
		args = append(args, strings.ToLower(occasion))
		i++
	}

	query += ` ORDER BY id ASC`

	if db.DB == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "database not connected"})
		return
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
		if err := rows.Scan(&p.ID, &p.Name, &p.Brand, &p.Category, &p.Gender, &p.Occasion, &p.Description, &p.ImageURL, &p.Price); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "row scan error: " + err.Error()})
			return
		}
		products = append(products, p)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "rows iteration error: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, products)
}

func GetProduct(c *gin.Context) {
	id := c.Param("id")

	var p models.Product
	err := db.DB.QueryRow(
		`SELECT id, name, COALESCE(brand,''), COALESCE(category,''), COALESCE(gender,''), COALESCE(occasion,''), COALESCE(description,''), COALESCE(image_url,''), COALESCE(price,0) FROM products WHERE id=$1`, id,
	).Scan(&p.ID, &p.Name, &p.Brand, &p.Category, &p.Gender, &p.Occasion, &p.Description, &p.ImageURL, &p.Price)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	rows, err := db.DB.Query(`SELECT id, product_id, COALESCE(size,''), COALESCE(price,0), COALESCE(stock,0) FROM product_variants WHERE product_id=$1 ORDER BY id ASC`, p.ID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	variants := []models.ProductVariant{}
	for rows.Next() {
		var v models.ProductVariant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Size, &v.Price, &v.Stock); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "variant scan error: " + err.Error()})
			return
		}
		variants = append(variants, v)
	}

	c.JSON(http.StatusOK, gin.H{"product": p, "variants": variants})
}

func CreateProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
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
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	id := c.Param("id")
	var p models.Product
	if err := c.ShouldBindJSON(&p); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := db.DB.Exec(
		`UPDATE products SET name=$1, brand=$2, category=$3, gender=$4, occasion=$5, description=$6, image_url=$7, price=$8 WHERE id=$9`,
		p.Name, p.Brand, p.Category, p.Gender, p.Occasion, p.Description, p.ImageURL, p.Price, id,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found or no changes made"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "updated"})
}

func DeleteProduct(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	id := c.Param("id")
	res, err := db.DB.Exec(`DELETE FROM products WHERE id=$1`, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "product not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "deleted"})
}

// ─── Product Variants (Admin) ────────────────────────────────────────────────

func CreateVariant(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	productID := c.Param("id")
	var v models.ProductVariant
	if err := c.ShouldBindJSON(&v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var id int
	err := db.DB.QueryRow(
		`INSERT INTO product_variants (product_id, size, price, stock) VALUES ($1,$2,$3,$4) RETURNING id`,
		productID, v.Size, v.Price, v.Stock,
	).Scan(&id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{"id": id})
}

func UpdateVariant(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	variantID := c.Param("variantId")
	var v models.ProductVariant
	if err := c.ShouldBindJSON(&v); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	res, err := db.DB.Exec(
		`UPDATE product_variants SET size=$1, price=$2, stock=$3 WHERE id=$4`,
		v.Size, v.Price, v.Stock, variantID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "variant not found or no changes made"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "variant updated"})
}

func DeleteVariant(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	variantID := c.Param("variantId")
	res, err := db.DB.Exec(`DELETE FROM product_variants WHERE id=$1`, variantID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil || rowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "variant not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "variant deleted"})
}

func GetVariants(c *gin.Context) {
	productID := c.Param("id")
	rows, err := db.DB.Query(`SELECT id, product_id, COALESCE(size,''), COALESCE(price,0), COALESCE(stock,0) FROM product_variants WHERE product_id=$1 ORDER BY id ASC`, productID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	variants := []models.ProductVariant{}
	for rows.Next() {
		var v models.ProductVariant
		if err := rows.Scan(&v.ID, &v.ProductID, &v.Size, &v.Price, &v.Stock); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "variant scan error: " + err.Error()})
			return
		}
		variants = append(variants, v)
	}

	c.JSON(http.StatusOK, variants)
}

// ─── Orders ──────────────────────────────────────────────────────────────────

func CreateOrder(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(int)

	var order models.Order
	if err := c.ShouldBindJSON(&order); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if len(order.Items) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "order items cannot be empty"})
		return
	}

	// Begin DB Transaction for atomic stock validation & order creation
	tx, err := db.DB.Begin()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to start transaction"})
		return
	}
	defer tx.Rollback()

	// 1. Validate Stock for all variant items before creating order
	for _, item := range order.Items {
		if item.VariantID > 0 {
			var currentStock int
			err := tx.QueryRow(`SELECT stock FROM product_variants WHERE id=$1 FOR UPDATE`, item.VariantID).Scan(&currentStock)
			if err != nil {
				if err == sql.ErrNoRows {
					c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("variant ID %d not found", item.VariantID)})
				} else {
					c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
				}
				return
			}
			if currentStock < item.Quantity {
				c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("insufficient stock for variant %d: available %d, requested %d", item.VariantID, currentStock, item.Quantity)})
				return
			}
		}
	}

	// 2. Create Razorpay Order
	razorpayOrderID, err := createRazorpayOrder(order.Total)
	if err != nil {
		log.Println("Razorpay order creation info:", err)
		razorpayOrderID = ""
	}

	// 3. Insert Order
	var orderID int
	err = tx.QueryRow(
		`INSERT INTO orders (user_id, total, address, razorpay_order_id, status) VALUES ($1,$2,$3,$4,'pending') RETURNING id`,
		userID, order.Total, order.Address, razorpayOrderID,
	).Scan(&orderID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 4. Insert Order Items and update stock inside transaction
	for _, item := range order.Items {
		_, err := tx.Exec(
			`INSERT INTO order_items (order_id, variant_id, product_id, size, quantity, price) VALUES ($1,$2,$3,$4,$5,$6)`,
			orderID, item.VariantID, item.ProductID, item.Size, item.Quantity, item.Price,
		)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		if item.VariantID > 0 {
			res, err := tx.Exec(`UPDATE product_variants SET stock=stock-$1 WHERE id=$2 AND stock >= $1`, item.Quantity, item.VariantID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "failed to deduct stock"})
				return
			}
			rowsAff, _ := res.RowsAffected()
			if rowsAff == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"error": "stock depletion race condition detected"})
				return
			}
		}
	}

	if err := tx.Commit(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "transaction commit failed"})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"order_id":          orderID,
		"razorpay_order_id": razorpayOrderID,
		"amount":            order.Total,
		"currency":          "INR",
		"razorpay_key":      os.Getenv("RAZORPAY_KEY_ID"),
	})
}

func VerifyPayment(c *gin.Context) {
	var req models.RazorpayVerifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	secret := os.Getenv("RAZORPAY_KEY_SECRET")
	if secret == "" {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "server configuration error: RAZORPAY_KEY_SECRET not configured"})
		return
	}

	// Verify signature server-side BEFORE marking order paid
	payload := req.OrderID + "|" + req.PaymentID
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(payload))
	expectedSignature := hex.EncodeToString(mac.Sum(nil))

	if expectedSignature != req.Signature {
		c.JSON(http.StatusBadRequest, gin.H{"error": "payment verification failed: invalid signature"})
		return
	}

	// Update order as paid
	res, err := db.DB.Exec(
		`UPDATE orders SET status='paid', razorpay_payment_id=$1 WHERE razorpay_order_id=$2`,
		req.PaymentID, req.OrderID,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAff, err := res.RowsAffected()
	if err != nil || rowsAff == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found for given razorpay_order_id"})
		return
	}

	// Get order details for Shiprocket
	var orderID int
	var address string
	var total float64
	err = db.DB.QueryRow(
		`SELECT id, address, total FROM orders WHERE razorpay_order_id=$1`, req.OrderID,
	).Scan(&orderID, &address, &total)
	if err == nil {
		// Attempt Shiprocket order creation ONLY after payment verification succeeds
		go createShiprocketOrder(orderID, address, total)
	}

	c.JSON(http.StatusOK, gin.H{"message": "payment verified", "status": "paid"})
}

func GetOrders(c *gin.Context) {
	userIDVal, exists := c.Get("user_id")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "unauthorized"})
		return
	}
	userID := userIDVal.(int)
	role, _ := c.Get("role")

	var rows *sql.Rows
	var err error

	if role == "admin" {
		rows, err = db.DB.Query(`SELECT id, user_id, status, total, COALESCE(address,''), COALESCE(razorpay_order_id,''), COALESCE(razorpay_payment_id,''), COALESCE(shipment_id,''), COALESCE(tracking_url,''), created_at FROM orders ORDER BY created_at DESC`)
	} else {
		rows, err = db.DB.Query(`SELECT id, user_id, status, total, COALESCE(address,''), COALESCE(razorpay_order_id,''), COALESCE(razorpay_payment_id,''), COALESCE(shipment_id,''), COALESCE(tracking_url,''), created_at FROM orders WHERE user_id=$1 ORDER BY created_at DESC`, userID)
	}

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	defer rows.Close()

	orders := []models.Order{}
	for rows.Next() {
		var o models.Order
		if err := rows.Scan(&o.ID, &o.UserID, &o.Status, &o.Total, &o.Address, &o.RazorpayOrderID, &o.RazorpayPaymentID, &o.ShipmentID, &o.TrackingURL, &o.CreatedAt); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "order scan error: " + err.Error()})
			return
		}
		orders = append(orders, o)
	}

	if err := rows.Err(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "order rows iteration error: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, orders)
}

// ─── Admin: Order Status ─────────────────────────────────────────────────────

func UpdateOrderStatus(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	id := c.Param("id")
	var body struct {
		Status string `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&body); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	validStatuses := map[string]bool{"pending": true, "paid": true, "confirmed": true, "shipped": true, "delivered": true}
	if !validStatuses[body.Status] {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid status"})
		return
	}

	res, err := db.DB.Exec(`UPDATE orders SET status=$1 WHERE id=$2`, body.Status, id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	rowsAff, err := res.RowsAffected()
	if err != nil || rowsAff == 0 {
		c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "status updated"})
}

// ─── Shiprocket ─────────────────────────────────────────────────────────────

func GetShipmentStatus(c *gin.Context) {
	role, exists := c.Get("role")
	if !exists || role != "admin" {
		c.JSON(http.StatusForbidden, gin.H{"error": "admin only"})
		return
	}

	orderID := c.Param("id")
	var shipmentID, trackingURL string
	err := db.DB.QueryRow(
		`SELECT COALESCE(shipment_id,''), COALESCE(tracking_url,'') FROM orders WHERE id=$1`, orderID,
	).Scan(&shipmentID, &trackingURL)
	if err != nil {
		if err == sql.ErrNoRows {
			c.JSON(http.StatusNotFound, gin.H{"error": "order not found"})
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	if shipmentID == "" {
		c.JSON(http.StatusOK, gin.H{"status": "not_shipped", "shipment_id": "", "tracking_url": ""})
		return
	}

	status, err := fetchShiprocketStatus(shipmentID)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"status": "unknown", "shipment_id": shipmentID, "tracking_url": trackingURL, "error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": status, "shipment_id": shipmentID, "tracking_url": trackingURL})
}

// ─── Razorpay Helpers ───────────────────────────────────────────────────────

func createRazorpayOrder(amount float64) (string, error) {
	keyID := os.Getenv("RAZORPAY_KEY_ID")
	keySecret := os.Getenv("RAZORPAY_KEY_SECRET")
	if keyID == "" || keySecret == "" {
		return "", fmt.Errorf("razorpay keys not configured")
	}

	amountPaise := int(amount * 100)
	payload := map[string]interface{}{
		"amount":   amountPaise,
		"currency": "INR",
		"receipt":  fmt.Sprintf("receipt_%d", time.Now().UnixNano()),
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", "https://api.razorpay.com/v1/orders", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.SetBasicAuth(keyID, keySecret)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	orderID, ok := result["id"].(string)
	if !ok {
		return "", fmt.Errorf("razorpay: invalid response: %v", result)
	}

	return orderID, nil
}

// ─── Shiprocket Helpers ─────────────────────────────────────────────────────

func getShiprocketToken() (string, error) {
	email := os.Getenv("SHIPROCKET_EMAIL")
	password := os.Getenv("SHIPROCKET_PASSWORD")
	if email == "" || password == "" {
		return "", fmt.Errorf("shiprocket credentials not configured")
	}

	payload := map[string]string{"email": email, "password": password}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post("https://apiv2.shiprocket.in/v1/external/auth/login", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	token, ok := result["token"].(string)
	if !ok {
		return "", fmt.Errorf("shiprocket: failed to get token")
	}

	return token, nil
}

func createShiprocketOrder(orderID int, address string, total float64) {
	token, err := getShiprocketToken()
	if err != nil {
		log.Println("Shiprocket auth failed:", err)
		return
	}

	rows, err := db.DB.Query(
		`SELECT oi.product_id, COALESCE(oi.size,''), oi.quantity, oi.price, COALESCE(p.name,'') FROM order_items oi LEFT JOIN products p ON oi.product_id=p.id WHERE oi.order_id=$1`, orderID,
	)
	if err != nil {
		log.Println("Shiprocket: failed to fetch order items:", err)
		return
	}
	defer rows.Close()

	items := []map[string]interface{}{}
	for rows.Next() {
		var productID, qty int
		var size, name string
		var price float64
		if err := rows.Scan(&productID, &size, &qty, &price, &name); err != nil {
			log.Println("Shiprocket: item scan error:", err)
			continue
		}
		items = append(items, map[string]interface{}{
			"name":           name,
			"sku":            fmt.Sprintf("PROD-%d-%s", productID, size),
			"units":          qty,
			"selling_price":  fmt.Sprintf("%.2f", price),
			"discount":       "0",
			"tax":            "0",
			"hsn":            "",
		})
	}

	payload := map[string]interface{}{
		"order_id":              fmt.Sprintf("HOS-%d", orderID),
		"order_date":           time.Now().Format("2006-01-02 15:04"),
		"pickup_location":     "Primary",
		"billing_customer_name":  "Customer",
		"billing_address":       address,
		"billing_city":          "City",
		"billing_pincode":       "000000",
		"billing_state":         "State",
		"billing_country":       "India",
		"billing_phone":         "0000000000",
		"shipping_is_billing":   true,
		"order_items":           items,
		"payment_method":        "Prepaid",
		"sub_total":             total,
		"length":               10,
		"breadth":              10,
		"height":               10,
		"weight":               0.5,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		log.Println("Shiprocket marshal error:", err)
		return
	}

	req, err := http.NewRequest("POST", "https://apiv2.shiprocket.in/v1/external/orders/create/adhoc", bytes.NewReader(body))
	if err != nil {
		log.Println("Shiprocket request creation error:", err)
		return
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Println("Shiprocket order creation request failed:", err)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	var result map[string]interface{}
	json.Unmarshal(respBody, &result)

	shipmentID := ""
	if sid, ok := result["shipment_id"]; ok {
		shipmentID = fmt.Sprintf("%v", sid)
	}

	trackingURL := ""
	if status, ok := result["status"].(string); ok && status != "" {
		trackingURL = fmt.Sprintf("https://shiprocket.co/tracking/%s", shipmentID)
	}

	if shipmentID != "" {
		db.DB.Exec(`UPDATE orders SET shipment_id=$1, tracking_url=$2 WHERE id=$3`, shipmentID, trackingURL, orderID)
		log.Printf("Shiprocket order created for HOS-%d, shipment: %s\n", orderID, shipmentID)
	} else {
		log.Printf("Shiprocket response for HOS-%d: %s\n", orderID, string(respBody))
	}
}

func fetchShiprocketStatus(shipmentID string) (string, error) {
	token, err := getShiprocketToken()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://apiv2.shiprocket.in/v1/external/courier/track/shipment/%s", shipmentID), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	if trackingData, ok := result["tracking_data"].(map[string]interface{}); ok {
		if status, ok := trackingData["shipment_status"].(string); ok {
			return status, nil
		}
	}

	return "unknown", nil
}