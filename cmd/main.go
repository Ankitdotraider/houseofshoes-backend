package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
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

	r.Use(cors.New(cors.Config{
		AllowOrigins: []string{"http://localhost:3000", "https://houseofshoes-frontend.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowCredentials: true,
	}))

	// Public routes
	r.POST("/signup", handlers.Signup)
	r.POST("/login", handlers.Login)
	r.GET("/products", handlers.GetProducts)
	r.GET("/products/:id", handlers.GetProduct)

	// Protected routes
	auth := r.Group("/")
	{
		auth.Use(middleware.AuthMiddleware())
		admin := auth.Group("/admin")
		{
			admin.POST("/products", handlers.CreateProduct)
			admin.PUT("/products/:id", handlers.UpdateProduct)
			admin.DELETE("/products/:id", handlers.DeleteProduct)
		}
		auth.POST("/orders", handlers.CreateOrder)
		auth.GET("/orders", handlers.GetOrders)
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}
	log.Fatal(r.Run(":" + port))
}
