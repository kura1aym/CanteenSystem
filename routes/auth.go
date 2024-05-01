package routes

import (
	"canteenSystem/controllers"
	"github.com/gin-gonic/gin"
)

func AuthRoutes(router *gin.Engine) {
	router.GET("/", controllers.WelcomePage)

	router.POST("/addNewMeal", controllers.AddNewMeal)

	router.POST("/login", controllers.Login)

	router.GET("/logout", controllers.Logout)

	router.POST("/register", controllers.Register)

	router.GET("/home", controllers.HomePage)

	router.GET("/categories", controllers.Categories)
}
