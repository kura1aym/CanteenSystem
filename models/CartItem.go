package models

type CartItem struct {
	ID         uint   `gorm:"primaryKey" json:"id"`
	UserID     uint   `json:"user_id"`
	ProductID  string `json:"product_id"`
	Product    Meal   `gorm:"foreignKey:ProductID;references:IDMeal" json:"product"`
	Quantity   int    `json:"quantity"`
	TotalPrice int    `json:"total_price"`
	OrderID    *uint  `json:"order_id"`
}

func (ci *CartItem) CalculateTotalPrice() {
	ci.TotalPrice = ci.Quantity * ci.Product.Price
}
