package routes

import (
	"canteenSystem/controllers"
	"github.com/gin-gonic/gin"
)

func AuthRoutes(router *gin.Engine) {
	router.POST("/login", controllers.Login)

	router.GET("/logout", controllers.Logout)

	router.POST("/register", controllers.Register)

	router.POST("/addNewMeal", controllers.AddNewMeal)

	router.GET("/home", controllers.HomePage)

	router.GET("/categories", controllers.Categories)

	router.GET("/cart", controllers.Cart)

	router.POST("/cart/add", controllers.AddToCart)

	router.POST("/cart/remove", controllers.RemoveFromCart)

	router.POST("/order", controllers.PlaceOrder)
	
	router.GET("/search", controllers.Search)
}
