package controllers

import (
	"canteenSystem/models"
	"canteenSystem/utils"
	"encoding/json"
	"errors"
	"fmt"
	"math/rand"
	"net/http"
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
	meals, err := GetMenuData()
	if err != nil {
		c.String(http.StatusInternalServerError, "Error fetching menu data: %v", err)
		return
	}
	c.HTML(http.StatusOK, "home.html", gin.H{
		"MenuItems":    meals,
		"LoggedInUser": loggedInUser,
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
		"https://www.themealdb.com/api/json/v1/1/search.php?f=c",
		// "https://themealdb.p.rapidapi.com/search.php?f=e",
		// "https://themealdb.p.rapidapi.com/search.php?f=b",
	}

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
			} else if err != nil {
				return nil, fmt.Errorf("database error: %w", err)
			}
		}

		allMeals = append(allMeals, mealsResp.Meals...)
	}

	allMeals = assignPrice()
	return allMeals, nil
}

func assignPrice() []models.Meal {
	for i := range allMeals {
		var existingMeal models.Meal
		err := models.DB.Where("id_meal = ?", allMeals[i].IDMeal).First(&existingMeal).Error
		if err == nil {
			allMeals[i].Price = existingMeal.Price
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			price, err := generateRandomPrice(1500, 3500)
			if err != nil {
				fmt.Println("Error generating price:", err)
				continue
			}
			allMeals[i].Price = price

			allMeals[i].Price = price
			existingMeal = allMeals[i]
			if err := models.DB.Create(&existingMeal).Error; err != nil {
				fmt.Println("Error saving meal to database:", err)
				continue
			}
		} else {
			fmt.Println("Error querying database:", err)
			continue
		}
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
		if cat.IdCategory == "2" || cat.IdCategory == "3" || cat.IdCategory == "13" {
			categories = append(categories, cat)
		}
	}
	for _, cat := range categories {
		fmt.Println(cat)
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
	err := models.DB.Where("user_id = ?", userID).
		Preload("Product").
		Find(&cartItems).Error
	if err != nil {
		return nil, err
	}
	return cartItems, nil
}

func AddToCart(c *gin.Context) {
	// Структура для хранения элементов корзины, полученных из запроса
	var cartItems []models.CartItem
	if err := c.ShouldBindJSON(&cartItems); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	for i, cartItem := range cartItems {
		// Получаем существующий элемент корзины, если он уже есть
		var existingCartItem models.CartItem
		err := models.DB.Where("user_id = ? AND product_id = ?", loggedInUser.ID, cartItem.ProductID).
			Preload("Product").
			First(&existingCartItem).Error

		// Если элемент уже существует в корзине, увеличиваем его количество и пересчитываем общую стоимость
		if err == nil {
			existingCartItem.Quantity += cartItem.Quantity
			existingCartItem.CalculateTotalPrice()
			if err := models.DB.Save(&existingCartItem).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось обновить товар в корзине"})
				return
			}
			cartItems[i] = existingCartItem
		} else if errors.Is(err, gorm.ErrRecordNotFound) {
			// Если элемент не найден в корзине, добавляем его как новый элемент
			err := models.DB.Where("id_meal = ?", cartItem.ProductID).
				First(&cartItem.Product).Error
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка базы данных"})
				return
			}

			// Устанавливаем идентификатор пользователя для элемента корзины
			cartItem.UserID = loggedInUser.ID

			// Установите `order_id` в `nil` или корректное значение
			cartItem.OrderID = nil // Установите в `nil` или в подходящее значение

			// Пересчитываем общую стоимость для нового элемента корзины
			cartItem.CalculateTotalPrice()

			// Добавляем новый элемент корзины в базу данных
			if err := models.DB.Create(&cartItem).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось добавить товар в корзину"})
				return
			}
			cartItems[i] = cartItem
		} else {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка базы данных"})
			return
		}
	}

	// Отправляем положительный ответ после добавления товаров в корзину
	c.JSON(http.StatusOK, gin.H{"success": "Товары добавлены в корзину", "cart_items": cartItems})
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
	err := models.DB.Where("user_id = ? AND product_id = ?", requestData.UserID, requestData.ProductID).
		Preload("Product").
		First(&cartItem).Error
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cart item not found"})
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
		Name    string `json:"name"`
		Email   string `json:"email"`
		Mobile  string `json:"mobile"`
		Street  string `json:"street"`
		City    string `json:"city"`
		State   string `json:"state"`
		Pincode string `json:"pincode"`
	}

	if err := c.ShouldBindJSON(&orderData); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	userID := loggedInUser.ID

	cartItems, err := GetCartItems(userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка получения данных о корзине"})
		return
	}

	totalCost := 0
	for _, item := range cartItems {
		totalCost += item.TotalPrice
	}

	order := models.Order{
		UserID:    userID,
		Name:      orderData.Name,
		Email:     orderData.Email,
		Mobile:    orderData.Mobile,
		Street:    orderData.Street,
		City:      orderData.City,
		State:     orderData.State,
		Pincode:   orderData.Pincode,
		TotalCost: totalCost,
		OrderDate: time.Now(),
		CartItems: cartItems, // Список товаров из корзины
	}

	if err := models.DB.Create(&order).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка сохранения заказа"})
		return
	}

	c.JSON(http.StatusOK, gin.H{"success": "Заказ успешно оформлен", "order": order})

	err = ClearCart(c)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Ошибка очистки корзины"})
		return
	}
}

func ClearCart(c *gin.Context) error {
	// Получаем идентификатор пользователя из текущей сессии или токена
	userID := loggedInUser.ID

	// Удаляем все товары из корзины пользователя по его идентификатору
	err := models.DB.Where("user_id = ?", userID).Delete(&models.CartItem{}).Error
	if err != nil {
		// Если возникает ошибка при удалении элементов из базы данных, возвращаем сообщение об ошибке
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Не удалось очистить корзину"})
		return nil
	}

	// Если корзина успешно очищена, возвращаем положительный ответ
	c.JSON(http.StatusOK, gin.H{"success": "Корзина успешно очищена"})
	return nil
}

func AddNewMeal(c *gin.Context) {
	fmt.Println("YOU ARE IN ADDTOMENU")
	var meal models.Meal

	if err := c.ShouldBindJSON(&meal); err != nil {
		c.JSON(400, gin.H{"error binding meal": err.Error()})
		return
	}

	result := models.DB.Where("StrMeal = ?", meal.StrMeal).First(&meal)
	if result.Error != nil && result.Error != gorm.ErrRecordNotFound {
		c.JSON(400, gin.H{"Error querying meal:": result.Error})
	}
	if result.Error == gorm.ErrRecordNotFound {
		if err := models.DB.Create(&meal).Error; err != nil {
			c.JSON(500, gin.H{"error": "could not create meal"})
			return
		}
		fmt.Println("Meal created successfully")
	} else {
		fmt.Println("Meal already exists")
	}

	allMeals = append(allMeals, meal)

	c.JSON(http.StatusOK, gin.H{"message": "Meal added successfully", "allMeals": meal})

}
