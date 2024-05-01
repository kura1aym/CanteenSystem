package models

type Meal struct {
	IDMeal       string `json:"idMeal" gorm:"unique"`
	StrMeal      string `json:"strMeal"`
	StrCategory  string `json:"strCategory"`
	StrMealThumb string `json:"strMealThumb"`
	Price        int
}
