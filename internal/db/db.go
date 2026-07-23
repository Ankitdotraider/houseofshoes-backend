package db

import (
	"database/sql"
	"log"
	"os"

	_ "github.com/lib/pq"
)

var DB *sql.DB

func Init() {
	var err error
	connStr := os.Getenv("DATABASE_URL")
	if connStr == "" {
		log.Fatal("DATABASE_URL not set")
	}

	DB, err = sql.Open("postgres", connStr)
	if err != nil {
		log.Fatal("Failed to connect to database:", err)
	}

	err = DB.Ping()
	if err != nil {
		log.Fatal("Database unreachable:", err)
	}

	log.Println("Database connected")
	createTables()
}

func createTables() {
	queries := []string{
		`CREATE TABLE IF NOT EXISTS users (
			id SERIAL PRIMARY KEY,
			email VARCHAR(255) UNIQUE NOT NULL,
			password VARCHAR(255) NOT NULL,
			role VARCHAR(50) DEFAULT 'customer',
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS products (
			id SERIAL PRIMARY KEY,
			name VARCHAR(255) NOT NULL,
			brand VARCHAR(255),
			category VARCHAR(100),
			gender VARCHAR(50),
			occasion VARCHAR(50),
			description TEXT,
			image_url TEXT,
			price DECIMAL(10,2),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		// Graceful migration for existing databases missing these columns
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS gender VARCHAR(50)`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS occasion VARCHAR(50)`,
		`ALTER TABLE products ADD COLUMN IF NOT EXISTS price DECIMAL(10,2)`,
		`CREATE TABLE IF NOT EXISTS product_variants (
			id SERIAL PRIMARY KEY,
			product_id INT REFERENCES products(id) ON DELETE CASCADE,
			size VARCHAR(20),
			price DECIMAL(10,2),
			stock INT DEFAULT 0
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			user_id INT REFERENCES users(id),
			status VARCHAR(50) DEFAULT 'pending',
			total DECIMAL(10,2),
			address TEXT,
			razorpay_order_id VARCHAR(255),
			razorpay_payment_id VARCHAR(255),
			shipment_id VARCHAR(255),
			tracking_url TEXT,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		// Graceful migration for existing orders table
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS razorpay_order_id VARCHAR(255)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS razorpay_payment_id VARCHAR(255)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS shipment_id VARCHAR(255)`,
		`ALTER TABLE orders ADD COLUMN IF NOT EXISTS tracking_url TEXT`,
		`CREATE TABLE IF NOT EXISTS order_items (
			id SERIAL PRIMARY KEY,
			order_id INT REFERENCES orders(id) ON DELETE CASCADE,
			variant_id INT REFERENCES product_variants(id),
			product_id INT,
			size VARCHAR(20),
			quantity INT,
			price DECIMAL(10,2)
		)`,
		// Graceful migration for existing order_items table
		`ALTER TABLE order_items ADD COLUMN IF NOT EXISTS product_id INT`,
		`ALTER TABLE order_items ADD COLUMN IF NOT EXISTS size VARCHAR(20)`,
	}

	for _, q := range queries {
		_, err := DB.Exec(q)
		if err != nil {
			log.Fatal("Failed to create table:", err)
		}
	}
	log.Println("Tables ready")
}