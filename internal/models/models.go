package models

import "time"

type User struct {
	ID        int       `json:"id"`
	Email     string    `json:"email"`
	Password  string    `json:"-"`
	Role      string    `json:"role"` // "admin" or "customer"
	CreatedAt time.Time `json:"created_at"`
}

type Product struct {
	ID          int       `json:"id"`
	Name        string    `json:"name"`
	Brand       string    `json:"brand"`
	Category    string    `json:"category"`
	Gender      string    `json:"gender"`
	Occasion    string    `json:"occasion"`
	Description string    `json:"description"`
	ImageURL    string    `json:"image_url"`
	Price       float64   `json:"price"`
	CreatedAt   time.Time `json:"created_at"`
}

type ProductVariant struct {
	ID        int     `json:"id"`
	ProductID int     `json:"product_id"`
	Size      string  `json:"size"`
	Price     float64 `json:"price"`
	Stock     int     `json:"stock"`
}

type Order struct {
	ID                 int         `json:"id"`
	UserID             int         `json:"user_id"`
	Status             string      `json:"status"` // pending, paid, confirmed, shipped, delivered
	Total              float64     `json:"total"`
	Address            string      `json:"address"`
	RazorpayOrderID    string      `json:"razorpay_order_id,omitempty"`
	RazorpayPaymentID  string      `json:"razorpay_payment_id,omitempty"`
	ShipmentID         string      `json:"shipment_id,omitempty"`
	TrackingURL        string      `json:"tracking_url,omitempty"`
	CreatedAt          time.Time   `json:"created_at"`
	Items              []OrderItem `json:"items"`
}

type OrderItem struct {
	ID        int     `json:"id"`
	OrderID   int     `json:"order_id"`
	VariantID int     `json:"variant_id"`
	ProductID int     `json:"product_id"`
	Size      string  `json:"size"`
	Quantity  int     `json:"quantity"`
	Price     float64 `json:"price"`
}

type LoginRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

type SignupRequest struct {
	Email    string `json:"email" binding:"required"`
	Password string `json:"password" binding:"required"`
}

// Razorpay
type RazorpayVerifyRequest struct {
	OrderID   string `json:"razorpay_order_id" binding:"required"`
	PaymentID string `json:"razorpay_payment_id" binding:"required"`
	Signature string `json:"razorpay_signature" binding:"required"`
}