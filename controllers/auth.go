package controllers

import (
	"canteenSystem/models"
	"canteenSystem/utils"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
	"strconv"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var todos []models.Todo
var loggedInUser models.User
var jwtKey = []byte("my_secret_key")
var allMeals []models.Meal

func WelcomePage(c *gin.Context) {
	fmt.Println("loggedInUser.ID + ", loggedInUser.ID)
	fmt.Println("loggedInUser.Role + ", loggedInUser.Role)
	fmt.Println("LoggedIn + ", loggedInUser.ID != 0)
	c.HTML(http.StatusOK, "index.html", gin.H{
		"Todos":    todos,
		"LoggedIn": loggedInUser.ID != 0,
		"Username": loggedInUser.Username,
		"Role":     loggedInUser.Role,
	})
}

func Register(c *gin.Context) {
	var user models.User

	if err := c.ShouldBind(&user); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var existingUser models.User
	if err := models.DB.Where("username = ?", user.Username).First(&existingUser).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(500, gin.H{"error": "database error"})
			return
		}
	}

	if existingUser.ID != 0 {
		c.JSON(400, gin.H{"error": "user already exists"})
		return
	}

	var errHash error
	user.Password, errHash = utils.GenerateHashPassword(user.Password)

	if errHash != nil {
		c.JSON(500, gin.H{"error": "could not generate password hash"})
		return
	}

	if err := models.DB.Create(&user).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not create user"})
		return
	}

	loggedInUser = user

	c.JSON(200, gin.H{"success": "user created"})
	fmt.Printf("Sign up loggedInUser: %+v\n", loggedInUser)
	fmt.Printf("Sign up user: %+v\n", user)

	c.Redirect(http.StatusSeeOther, "/home")
}

func Login(c *gin.Context) {
	var user models.User

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	models.DB.Where("username = ?", user.Username).First(&loggedInUser)

	if loggedInUser.ID == 0 {
		c.JSON(400, gin.H{"error": "user does not exist"})
		return
	}

	errHash := utils.CompareHashPassword(user.Password, loggedInUser.Password)

	if !errHash {
		c.JSON(400, gin.H{"error": "invalid password"})
		return
	}

	fmt.Printf("Login loggedInUser: %+v\n", loggedInUser)
	fmt.Printf("Login user: %+v\n", user)

	expirationTime := time.Now().Add(5 * time.Minute)

	claims := &models.Claims{
		Role: loggedInUser.Role,
		StandardClaims: jwt.StandardClaims{
			Subject:   loggedInUser.Email,
			ExpiresAt: expirationTime.Unix(),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	tokenString, err := token.SignedString(jwtKey)

	if err != nil {
		c.JSON(500, gin.H{"error": "could not generate token"})
		return
	}

	c.SetCookie("token", tokenString, int(expirationTime.Unix()), "/", "localhost", false, true)
	c.JSON(200, gin.H{"success": "user logged in"})
	c.Redirect(http.StatusSeeOther, "/home")
}

func AddToDo(c *gin.Context) {
	fmt.Println("YOU ARE IN ADDTODO")
	var todo models.Todo

	if err := c.ShouldBindJSON(&todo); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	todo.UserID = int(loggedInUser.ID)
	fmt.Println("Added task ", todo)
	fmt.Println("Added task ", loggedInUser.ID)

	if err := models.DB.Create(&todo).Error; err != nil {
		c.JSON(500, gin.H{"error": "could not create todo"})
		return
	}

	fmt.Println("Added task ", todo)

	todos = append(todos, todo)

	fmt.Println("Added tasks ", todos)
	c.JSON(http.StatusOK, gin.H{"message": "Task added successfully", "todo": todo})
}

func Toggle(c *gin.Context) {
	index := c.PostForm("index")
	toggleIndex(index)
	c.Redirect(http.StatusSeeOther, "/")
}

func Logout(c *gin.Context) {
	loggedInUser = models.User{}
	fmt.Println("YOU ARE HERELOGOUT", loggedInUser)
	c.SetCookie("token", "", -1, "/", "localhost", false, true)
	c.JSON(200, gin.H{"success": "user logged out"})
	c.Redirect(http.StatusSeeOther, "/")
}

func toggleIndex(index string) {
	i, _ := strconv.Atoi(index)
	if i >= 0 && i < len(todos) {
		todos[i].Done = !todos[i].Done
	}
}

func HomePage(c *gin.Context) {
	meals, err := GetMenuData()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching menu data: %v", err)
		return
	}
	c.HTML(http.StatusOK, "home.html", gin.H{
		"MenuItems": meals,
		"Username":  loggedInUser.Username,
	})
}

type MealsResponse struct {
	Meals []models.Meal `json:"meals"`
}

func generateRandomPrice(min, max int) (int, error) {
	if min >= max {
		return 0, errors.New("min must be less than max")
	}
	return rand.Intn(max-min) + min, nil
}

func GetMenuData() ([]models.Meal, error) {
	urls := []string{
		"https://themealdb.p.rapidapi.com/search.php?f=c",
		// "https://themealdb.p.rapidapi.com/search.php?f=e",
		// "https://themealdb.p.rapidapi.com/search.php?f=b",
	}

	for _, url := range urls {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add("X-RapidAPI-Key", "f4269533cemshc241dafc079c688p1e6f04jsnd66e347907f3")
		req.Header.Add("X-RapidAPI-Host", "themealdb.p.rapidapi.com")

		client := &http.Client{}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		defer resp.Body.Close()

		var mealsResp MealsResponse
		if err := json.NewDecoder(resp.Body).Decode(&mealsResp); err != nil {
			return nil, err
		}

		if len(mealsResp.Meals) > 10 {
			mealsResp.Meals = mealsResp.Meals[:10]
		}

		allMeals = append(allMeals, mealsResp.Meals...)
	}

	allMeals = assignPrice()
	return allMeals, nil
}

func assignPrice() []models.Meal {
	for i := range allMeals {
		price, err := generateRandomPrice(1500, 3500)
		if err != nil {
			return nil
		}
		allMeals[i].Price = price
	}
	return allMeals
}

type CategoriesResponse struct {
	Categories []models.Category `json:"categories"`
}

func Categories(c *gin.Context) {
	categories, err := GetCategories()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching menu data: %v", err)
		return
	}
	c.HTML(http.StatusOK, "categories.html", gin.H{
		"Categories": categories,
		"Username":   loggedInUser.Username,
	})
}

func GetCategories() ([]models.Category, error) {

	url := "https://themealdb.p.rapidapi.com/categories.php"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("X-RapidAPI-Key", "f4269533cemshc241dafc079c688p1e6f04jsnd66e347907f3")
	req.Header.Add("X-RapidAPI-Host", "themealdb.p.rapidapi.com")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()
	var categoriesResp CategoriesResponse
	if err := json.NewDecoder(resp.Body).Decode(&categoriesResp); err != nil {
		return nil, err
	}
	var categories []models.Category
	for _, cat := range categoriesResp.Categories {
		if cat.IdCategory == "2" || cat.IdCategory == "3" || cat.IdCategory == "13" {
			categories = append(categories, cat)
		}
	}
	for _, cat := range categories {
		fmt.Println(cat)
	}

	return categories, nil
}
