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
	"strings"
	"time"

	"github.com/dgrijalva/jwt-go"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

var loggedInUser models.User
var jwtKey = []byte("my_secret_key")
var allMeals []models.Meal

func Register(c *gin.Context) {
	var user models.User

	if err := c.ShouldBind(&user); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	var existingUser models.User
	if err := models.DB.Where("username = ? OR email = ?", user.Username, user.Email).First(&existingUser).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "database error"})
			return
		}
	}

	if existingUser.ID != 0 {
		if existingUser.Username == user.Username {
			c.JSON(http.StatusBadRequest, gin.H{"error": "username already in use"})
		} else if existingUser.Email == user.Email {
			c.JSON(http.StatusBadRequest, gin.H{"error": "email already in use"})
		}
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

func Logout(c *gin.Context) {
	loggedInUser = models.User{}
	c.SetCookie("token", "", -1, "/", "localhost", false, true)
	c.JSON(200, gin.H{"success": "user logged out"})
	c.Redirect(http.StatusSeeOther, "login.html")
}

func HomePage(c *gin.Context) {
	category := c.Query("category")
  	if category != "" {
    	OneCategory(c)
  	}else {
		_, err := GetMenuData()
		if err != nil {
			c.String(http.StatusInternalServerError, "Error fetching menu data: %v", err)
			return
		}
		var mealsLocal []models.Meal

		err = models.DB.Find(&mealsLocal).Error
		if err != nil {
			c.String(http.StatusInternalServerError, "Error fetching meals data: %v", err)
		}

		c.HTML(http.StatusOK, "home.html", gin.H{
			"AllMeals":    mealsLocal,
			"LoggedInUser": loggedInUser,
		})
	}
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
		"https://www.themealdb.com/api/json/v1/1/search.php?f=c",
		"https://www.themealdb.com/api/json/v1/1/search.php?f=e",
		"https://www.themealdb.com/api/json/v1/1/search.php?f=f",
	}

	var meals []models.Meal

	for _, url := range urls {
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			return nil, err
		}

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

		for _, meal := range mealsResp.Meals {
			var existingMeal models.Meal
			err := models.DB.Where("id_meal = ?", meal.IDMeal).First(&existingMeal).Error

			if errors.Is(err, gorm.ErrRecordNotFound) {
				if err := models.DB.Create(&meal).Error; err != nil {
					return nil, fmt.Errorf("could not add meal to database: %w", err)
				}

				price, err := generateRandomPrice(1500, 3500)
				if err != nil {
					return nil, fmt.Errorf("error generating price: %w", err)
				}
				meal.Price = price

				if err := models.DB.Save(&meal).Error; err != nil {
					return nil, fmt.Errorf("error saving meal to database: %w", err)
				}

				meals = append(meals, meal)
			} else if err != nil {
				return nil, fmt.Errorf("database error: %w", err)
			} else {
				if existingMeal.Price != 0 {
					meal.Price = existingMeal.Price
				}
				meals = append(meals, meal)
			}
		}
	}

	return meals, nil
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
	url := "https://www.themealdb.com/api/json/v1/1/categories.php"
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

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
		categories = append(categories, cat)
	}

	return categories, nil
}

func Cart(c *gin.Context) {
	userID := loggedInUser.ID

	cartItems, err := GetCartItems(userID)
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching cart data: %v", err)
		return
	}

	totalCost := 0
	foodItems := 0
	for _, item := range cartItems {
		totalCost += item.TotalPrice
		foodItems += item.Quantity
	}

	cartEmpty := len(cartItems) == 0

	c.HTML(http.StatusOK, "cart.html", gin.H{
		"CartItems":    cartItems,
		"TotalCost":    totalCost,
		"LoggedInUser": loggedInUser,
		"FoodItems":    foodItems,
		"CartEmpty":    cartEmpty,
	})
}

func GetCartItems(userID uint) ([]models.CartItem, error) {
	var cartItems []models.CartItem
	err := models.DB.Where("user_id = ? AND order_id IS NULL", userID).
		Preload("Product").
		Find(&cartItems).Error
	if err != nil {
		return nil, err
	}
	return cartItems, nil
}

func AddToCart(c *gin.Context) {
	var cartItems []models.CartItem
	if err := c.ShouldBindJSON(&cartItems); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for i, cartItem := range cartItems {
		var existingCartItem models.CartItem
		err := models.DB.Where("user_id = ? AND product_id = ?", loggedInUser.ID, cartItem.ProductID).
			Preload("Product").
			First(&existingCartItem).Error

		if err == nil {
			existingCartItem.Quantity += cartItem.Quantity
			existingCartItem.CalculateTotalPrice()
			if err := models.DB.Save(&existingCartItem).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to update the item in the shopping cart"})
				return
			}
			cartItems[i] = existingCartItem
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			err := models.DB.Where("id_meal = ?", cartItem.ProductID).
				First(&cartItem.Product).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
				return
			}

			cartItem.UserID = loggedInUser.ID

			cartItem.OrderID = nil

			cartItem.CalculateTotalPrice()

			if err := models.DB.Create(&cartItem).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "The product could not be added to the cart"})
				return
			}
			cartItems[i] = cartItem
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Database error"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": "Products added to the cart", "cart_items": cartItems})
}

func RemoveFromCart(c *gin.Context) {
	var requestData struct {
		UserID    uint   `json:"user_id"`
		ProductID string `json:"product_id"`
	}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var cartItem models.CartItem
	err := models.DB.Where("user_id = ? AND product_id = ? AND order_id IS NULL", requestData.UserID, requestData.ProductID).
		Preload("Product").
		First(&cartItem).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "cart item not found"})
		return
	}

	if cartItem.Quantity > 1 {
		cartItem.Quantity--
		cartItem.CalculateTotalPrice()
		if err := models.DB.Save(&cartItem).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update cart item"})
			return
		}
	} else {
		if err := models.DB.Delete(&cartItem).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete cart item"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": "cart item removed"})
}

func PlaceOrder(c *gin.Context) {
	var orderData struct {
		Name     string  `json:"name"`
		Email    string  `json:"email"`
		Mobile   string  `json:"mobile"`
		Street   string  `json:"street"`
		City     string  `json:"city"`
		State    string  `json:"state"`
		Pincode  string  `json:"pincode"`
		Discount float64 `json:"discount"`
	}

	if err := c.ShouldBindJSON(&orderData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := loggedInUser.ID

	cartItems, err := GetCartItemsWithoutOrderID(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error fetching cart data"})
		return
	}

	originalTotalCost := 0
	for _, item := range cartItems {
		originalTotalCost += item.TotalPrice
	}

	finalTotalCost := float64(originalTotalCost) - orderData.Discount

	order := models.Order{
		UserID:           userID,
		Name:             orderData.Name,
		Email:            orderData.Email,
		Mobile:           orderData.Mobile,
		Street:           orderData.Street,
		City:             orderData.City,
		State:            orderData.State,
		Pincode:          orderData.Pincode,
		TotalCost:        originalTotalCost,
		Discount:         orderData.Discount,
		CostWithDiscount: finalTotalCost,
		OrderDate:        time.Now(),
		CartItems:        cartItems,
	}

	if err := models.DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error saving order"})
		return
	}

	for _, item := range cartItems {
		item.OrderID = &order.ID
		if err := models.DB.Save(&item).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Error updating cart item"})
			return
		}
	}

	c.JSON(http.StatusOK, gin.H{"success": "Order placed successfully", "order": order})
}

func GetCartItemsWithoutOrderID(userID uint) ([]models.CartItem, error) {
	var cartItems []models.CartItem
	err := models.DB.Where("user_id = ? AND order_id IS NULL", userID).
		Preload("Product").
		Find(&cartItems).Error
	if err != nil {
		return nil, err
	}
	return cartItems, nil
}

func OrderList(c *gin.Context) {
	userID := loggedInUser.ID

	var orders []models.Order
	err := models.DB.Where("user_id = ?", userID).Preload("CartItems.Product").Find(&orders).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Error receiving orders"})
		return
	}

	fmt.Println("ORDERS: ", orders)

	c.HTML(http.StatusOK, "order_list.html", gin.H{
		"orders":       orders,
		"LoggedInUser": loggedInUser,
	})
}

func Search(c *gin.Context) {
	searchTerm := c.Query("s")
	searchResult := SearchResult(searchTerm)

	if searchTerm == "" {
		c.HTML(http.StatusOK, "search.html", gin.H{
			"SearchResult": nil,
			"Username":     loggedInUser.Username,
		})
	} else {
		c.HTML(http.StatusOK, "search.html", gin.H{
			"SearchResult": searchResult,
			"Username":     loggedInUser.Username,
		})
	}
}

func SearchResult(searchTerm string) []models.Meal {
	var searchResult []models.Meal
	searchTerm = strings.ToLower(searchTerm)
	result := models.DB.Where("LOWER(str_meal) LIKE ?", "%"+searchTerm+"%").Find(&searchResult)
	if result.Error != nil {
		return nil
	}
	return searchResult
}

func Admin(c *gin.Context){
	id := c.Query("mealID")
	categories, err := GetCategories()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching menu data: %v", err)
		return
	}
	var meals []models.Meal
	
	if id ==""{
		var edit bool = false
		err = models.DB.Find(&meals).Error
		if err != nil {
			c.String(http.StatusInternalServerError, "Error fetching meals data: %v", err)
		}
		c.HTML(http.StatusOK, "admin.html", gin.H{
			"Edit": edit,
			"Categories":   categories,
			"AllMeals":    meals,
			"LoggedInUser": loggedInUser,
		})
	}else{
		var meal models.Meal
		var edit bool = true
		err := models.DB.Where("id_meal = ?", id).First(&meal).Error
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "meal item not found"})
			return
		}
		if err := models.DB.Where("str_category = ?", meal.StrCategory).Delete(&categories).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not retrive category"})
			return
		}
		c.HTML(http.StatusOK, "admin.html", gin.H{
			"Edit": edit,
			"Categories":   categories,
			"AllMeals":    meal,
			"LoggedInUser": loggedInUser,
			"Category": meal.StrCategory,
		})
	}

}

func PlaceMeal(c *gin.Context){
	var meal models.Meal
	var mealData struct{
		Name      string `json:"name"`
		Img       string `json:"url"`
		Category  string `json:"category"` 
		Price     int    `json:"price"`
	}

	if err := c.ShouldBindJSON(&mealData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := models.DB.Where("str_meal = ? AND str_category = ?", mealData.Name, mealData.Category).First(&meal)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		c.JSON(http.StatusOK, gin.H{"message": "Meal already exists"})
	} 
	if result.Error == gorm.ErrRecordNotFound {
		i :=  strconv.Itoa(getID())
		meal = models.Meal{
	        IDMeal:         i,
			StrMeal:       mealData.Name,
			StrCategory:   mealData.Category,
			StrMealThumb:  mealData.Img,
			Price:         mealData.Price,
		}
		if err := models.DB.Create(&meal).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "could not create meal"})
			return
		}
		allMeals = append(allMeals, meal)
		c.JSON(http.StatusOK, gin.H{"message": "Meal added successfully", "meal": meal})
	}
}

var random *rand.Rand
const maxID = 52776
func init() {
    random = rand.New(rand.NewSource(time.Now().UnixNano()))
}

func getID() int {
    existingIDs := make(map[int]bool)

    for _, meal := range allMeals {
        
        id, err := strconv.Atoi(meal.IDMeal)
        if err == nil {
            existingIDs[id] = true
        }
    }

   
    var id int
    for {
        id = random.Intn(maxID) + 1 
        if !existingIDs[id] {
            break 
        }
    }

    return id
}

func OneCategory(c *gin.Context) {
	category := c.Query("category")
	var categoryResult []models.Meal
	category = strings.ToLower(category)
	models.DB.Where("LOWER(str_category) LIKE ?", "%"+category+"%").Find(&categoryResult)
	c.HTML(http.StatusOK, "home.html", gin.H{
		"AllMeals": categoryResult,
		"Username":     loggedInUser.Username,
	})
}

func RemoveMeal(c *gin.Context) {
	var requestData struct {
		ID string `json:"product_id"`
	}
	if err := c.ShouldBindJSON(&requestData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	var meal models.Meal
	err := models.DB.Where("id_meal = ?", requestData.ID).First(&meal).Error
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "meal item not found"})
		return
	}

	if err := models.DB.Delete(&meal).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not delete meal item"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": "meal item removed"})
}


func UpdateMeal(c *gin.Context){
	var meal models.Meal
	var mealData struct{
		Name      string `json:"name"`
		Img       string `json:"url"`
		Category  string `json:"category"` 
		Price     int    `json:"price"`
		ID        string `json:"product_id"`       
	}

	if err := c.ShouldBindJSON(&mealData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result := models.DB.Where("id_meal = ?", mealData.ID).First(&meal)
	if result.Error != nil {
		c.JSON(http.StatusOK, gin.H{"message": "Meal doesn't exists"})
	} 
	meal = models.Meal{
		StrMeal:       mealData.Name,
		StrCategory:   mealData.Category,
		StrMealThumb:  mealData.Img,
		Price:         mealData.Price,
	}
	if err := models.DB.Model(&models.Meal{}).Where("id_meal = ?", mealData.ID).Updates(map[string]interface{}{"str_meal": mealData.Name, "str_category": mealData.Category, "str_meal_thumb":mealData.Img, "price": mealData.Price}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update meal"})
		return
	}
	if err := models.DB.Model(&allMeals).Where("id_meal = ?", mealData.ID).Updates(map[string]interface{}{"str_meal": mealData.Name, "str_category": mealData.Category, "str_meal_thumb":mealData.Img, "price": mealData.Price}).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "could not update allMeals"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "Meal added successfully", "meal": meal})
	}
