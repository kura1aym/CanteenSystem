package models

type Meal struct {
	IDMeal       string `json:"idMeal" gorm:"primaryKey"`
	StrMeal      string `json:"strMeal"`
	StrCategory  string `json:"strCategory"`
	StrMealThumb string `json:"strMealThumb"`
	Price        int    `json:"price"`
}
