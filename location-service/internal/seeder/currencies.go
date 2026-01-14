package seeder

import (
	"location-service/internal/models"
)

func getCurrenciesData() []models.Currency {
	return []models.Currency{
		{Code: "USD", Name: "United States Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "EUR", Name: "Euro", Symbol: "€", DecimalPlaces: 2, Active: true},
		{Code: "GBP", Name: "British Pound", Symbol: "£", DecimalPlaces: 2, Active: true},
		{Code: "JPY", Name: "Japanese Yen", Symbol: "¥", DecimalPlaces: 0, Active: true},
		{Code: "CAD", Name: "Canadian Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "AUD", Name: "Australian Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "CHF", Name: "Swiss Franc", Symbol: "CHF", DecimalPlaces: 2, Active: true},
		{Code: "CNY", Name: "Chinese Yuan", Symbol: "¥", DecimalPlaces: 2, Active: true},
		{Code: "INR", Name: "Indian Rupee", Symbol: "₹", DecimalPlaces: 2, Active: true},
		{Code: "SGD", Name: "Singapore Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "NZD", Name: "New Zealand Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "ZAR", Name: "South African Rand", Symbol: "R", DecimalPlaces: 2, Active: true},
		{Code: "AED", Name: "UAE Dirham", Symbol: "د.إ", DecimalPlaces: 2, Active: true},
		{Code: "BRL", Name: "Brazilian Real", Symbol: "R$", DecimalPlaces: 2, Active: true},
		{Code: "MXN", Name: "Mexican Peso", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "KRW", Name: "South Korean Won", Symbol: "₩", DecimalPlaces: 0, Active: true},
		{Code: "SEK", Name: "Swedish Krona", Symbol: "kr", DecimalPlaces: 2, Active: true},
	}
}
