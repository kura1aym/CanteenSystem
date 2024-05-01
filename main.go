package main

import (
	"canteenSystem/controllers"
	"canteenSystem/models"
	"canteenSystem/routes"
	"github.com/gin-gonic/gin"
	"net/http"
	"fmt"
	"log"
	"os"
	"github.com/joho/godotenv"
)

func main() {
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}
	config := models.Config{
		Host:     os.Getenv("DB_HOST"),
		Port:     os.Getenv("DB_PORT"),
		User:     os.Getenv("DB_USER"),
		Password: os.Getenv("DB_PASSWORD"),
		DBName:   os.Getenv("DB_NAME"),
		SSLMode:  os.Getenv("DB_SSLMODE"),
	}

	models.InitDB(config)
	// check()
	router := gin.Default()
	fs := http.FileServer(http.Dir("assets"))
	router.GET("/assets/*filepath", func(c *gin.Context) {
		http.StripPrefix("/assets/", fs).ServeHTTP(c.Writer, c.Request)
	})

	router.LoadHTMLGlob("templates/*")
	router.StaticFile("/login.html", "./templates/login.html")

	routes.AuthRoutes(router)

	router.Run(":8080")
}

func check() {
	meals, err := controllers.GetMenuData()
	if err != nil {
		fmt.Println("Error fetching menu data:", err)
		return
	}
	// Print fetched menu data
	fmt.Println("Menu Items:")
	for _, meal := range meals {
		fmt.Printf("ID: %s, Meal: %s, Category: %s\n", meal.IDMeal, meal.StrMeal, meal.StrCategory)
	}
}
