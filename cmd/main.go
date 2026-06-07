package main

import (
	"log"
	"os"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"github.com/Ankitdotraider/houseofshoes/internal/db"
	"github.com/Ankitdotraider/houseofshoes/internal/handlers"
	"github.com/Ankitdotraider/houseofshoes/internal/middleware"
)

func main() {
	godotenv.Load()

	db.Init()

	r := gin.Default()

	// Public routes
	r.POST("/signup", handlers.Signup)
	r.POST("/login", handlers.Login)
	r.GET("/products", handlers.GetProducts)
	r.GET("/products/:id", handlers.GetProduct)

	// Protected routes
	auth := r.Group("/")
	auth.Use(middleware.AuthMiddleware())
	{
		// Admin
		auth.POST("/admin/products", handlers.CreateProduct)
		auth.PUT("/admin/products/:id", handlers.UpdateProduct)
		auth.DELETE("/admin/products/:id", handlers.DeleteProduct)

		// Orders
		auth.POST("/orders", handlers.CreateOrder)
		auth.GET("/orders", handlers.GetOrders)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(r.Run(":" + port))
}