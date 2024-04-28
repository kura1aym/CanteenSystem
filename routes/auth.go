package routes

import (
	"canteenSystem/controllers"
	"github.com/gin-gonic/gin"
)

func AuthRoutes(router *gin.Engine) {
	router.GET("/", controllers.WelcomePage)

	router.POST("/add", controllers.AddToDo)

	router.POST("/toggle", controllers.Toggle)

	router.POST("/login", controllers.Login)

	router.GET("/logout", controllers.Logout)

	router.POST("/register", controllers.Register)
}
