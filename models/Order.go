package models

import (
	"time"
)

type Order struct {
	ID               uint       `gorm:"primaryKey" json:"id"`
	UserID           uint       `json:"user_id"`
	Name             string     `json:"name"`
	Email            string     `json:"email"`
	Mobile           string     `json:"mobile"`
	Street           string     `json:"street"`
	City             string     `json:"city"`
	State            string     `json:"state"`
	Pincode          string     `json:"pincode"`
	TotalCost        int        `json:"total_cost"`
	Discount         float64    `json:"discount"`
	CostWithDiscount float64    `json:"cost_with_discount"`
	OrderDate        time.Time  `json:"order_date"`
	CartItems        []CartItem `gorm:"foreignKey:OrderID;constraint:OnDelete:CASCADE" json:"cart_items"`
}
