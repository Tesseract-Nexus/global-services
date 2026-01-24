package seeder

import (
	"location-service/internal/models"
)

func getCurrenciesData() []models.Currency {
	return []models.Currency{
		// Major World Currencies
		{Code: "USD", Name: "United States Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "EUR", Name: "Euro", Symbol: "€", DecimalPlaces: 2, Active: true},
		{Code: "GBP", Name: "British Pound", Symbol: "£", DecimalPlaces: 2, Active: true},
		{Code: "JPY", Name: "Japanese Yen", Symbol: "¥", DecimalPlaces: 0, Active: true},
		{Code: "CAD", Name: "Canadian Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "AUD", Name: "Australian Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "CHF", Name: "Swiss Franc", Symbol: "CHF", DecimalPlaces: 2, Active: true},
		{Code: "CNY", Name: "Chinese Yuan", Symbol: "¥", DecimalPlaces: 2, Active: true},

		// South Asian Currencies
		{Code: "INR", Name: "Indian Rupee", Symbol: "₹", DecimalPlaces: 2, Active: true},
		{Code: "NPR", Name: "Nepalese Rupee", Symbol: "₨", DecimalPlaces: 2, Active: true},
		{Code: "BDT", Name: "Bangladeshi Taka", Symbol: "৳", DecimalPlaces: 2, Active: true},
		{Code: "LKR", Name: "Sri Lankan Rupee", Symbol: "₨", DecimalPlaces: 2, Active: true},
		{Code: "PKR", Name: "Pakistani Rupee", Symbol: "₨", DecimalPlaces: 2, Active: true},
		{Code: "AFN", Name: "Afghan Afghani", Symbol: "؋", DecimalPlaces: 2, Active: true},
		{Code: "BTN", Name: "Bhutanese Ngultrum", Symbol: "Nu.", DecimalPlaces: 2, Active: true},
		{Code: "MVR", Name: "Maldivian Rufiyaa", Symbol: "Rf", DecimalPlaces: 2, Active: true},

		// Southeast Asian Currencies
		{Code: "SGD", Name: "Singapore Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "IDR", Name: "Indonesian Rupiah", Symbol: "Rp", DecimalPlaces: 0, Active: true},
		{Code: "PHP", Name: "Philippine Peso", Symbol: "₱", DecimalPlaces: 2, Active: true},
		{Code: "THB", Name: "Thai Baht", Symbol: "฿", DecimalPlaces: 2, Active: true},
		{Code: "VND", Name: "Vietnamese Dong", Symbol: "₫", DecimalPlaces: 0, Active: true},
		{Code: "MYR", Name: "Malaysian Ringgit", Symbol: "RM", DecimalPlaces: 2, Active: true},
		{Code: "MMK", Name: "Myanmar Kyat", Symbol: "K", DecimalPlaces: 2, Active: true},
		{Code: "KHR", Name: "Cambodian Riel", Symbol: "៛", DecimalPlaces: 2, Active: true},
		{Code: "LAK", Name: "Lao Kip", Symbol: "₭", DecimalPlaces: 2, Active: true},
		{Code: "BND", Name: "Brunei Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},

		// East Asian Currencies
		{Code: "KRW", Name: "South Korean Won", Symbol: "₩", DecimalPlaces: 0, Active: true},
		{Code: "TWD", Name: "New Taiwan Dollar", Symbol: "NT$", DecimalPlaces: 2, Active: true},
		{Code: "HKD", Name: "Hong Kong Dollar", Symbol: "HK$", DecimalPlaces: 2, Active: true},
		{Code: "MOP", Name: "Macanese Pataca", Symbol: "MOP$", DecimalPlaces: 2, Active: true},
		{Code: "MNT", Name: "Mongolian Tugrik", Symbol: "₮", DecimalPlaces: 2, Active: true},

		// Middle East Currencies
		{Code: "AED", Name: "UAE Dirham", Symbol: "د.إ", DecimalPlaces: 2, Active: true},
		{Code: "SAR", Name: "Saudi Riyal", Symbol: "﷼", DecimalPlaces: 2, Active: true},
		{Code: "QAR", Name: "Qatari Riyal", Symbol: "﷼", DecimalPlaces: 2, Active: true},
		{Code: "KWD", Name: "Kuwaiti Dinar", Symbol: "د.ك", DecimalPlaces: 3, Active: true},
		{Code: "BHD", Name: "Bahraini Dinar", Symbol: "BD", DecimalPlaces: 3, Active: true},
		{Code: "OMR", Name: "Omani Rial", Symbol: "﷼", DecimalPlaces: 3, Active: true},
		{Code: "ILS", Name: "Israeli Shekel", Symbol: "₪", DecimalPlaces: 2, Active: true},
		{Code: "TRY", Name: "Turkish Lira", Symbol: "₺", DecimalPlaces: 2, Active: true},

		// European Currencies
		{Code: "SEK", Name: "Swedish Krona", Symbol: "kr", DecimalPlaces: 2, Active: true},
		{Code: "NOK", Name: "Norwegian Krone", Symbol: "kr", DecimalPlaces: 2, Active: true},
		{Code: "DKK", Name: "Danish Krone", Symbol: "kr", DecimalPlaces: 2, Active: true},
		{Code: "PLN", Name: "Polish Zloty", Symbol: "zł", DecimalPlaces: 2, Active: true},
		{Code: "RUB", Name: "Russian Ruble", Symbol: "₽", DecimalPlaces: 2, Active: true},

		// Americas Currencies
		{Code: "BRL", Name: "Brazilian Real", Symbol: "R$", DecimalPlaces: 2, Active: true},
		{Code: "MXN", Name: "Mexican Peso", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "ARS", Name: "Argentine Peso", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "CLP", Name: "Chilean Peso", Symbol: "$", DecimalPlaces: 0, Active: true},
		{Code: "COP", Name: "Colombian Peso", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "PEN", Name: "Peruvian Sol", Symbol: "S/", DecimalPlaces: 2, Active: true},

		// Africa Currencies
		{Code: "ZAR", Name: "South African Rand", Symbol: "R", DecimalPlaces: 2, Active: true},
		{Code: "EGP", Name: "Egyptian Pound", Symbol: "£", DecimalPlaces: 2, Active: true},
		{Code: "NGN", Name: "Nigerian Naira", Symbol: "₦", DecimalPlaces: 2, Active: true},
		{Code: "KES", Name: "Kenyan Shilling", Symbol: "KSh", DecimalPlaces: 2, Active: true},
		{Code: "GHS", Name: "Ghanaian Cedi", Symbol: "₵", DecimalPlaces: 2, Active: true},
		{Code: "MAD", Name: "Moroccan Dirham", Symbol: "MAD", DecimalPlaces: 2, Active: true},

		// Oceania
		{Code: "NZD", Name: "New Zealand Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "FJD", Name: "Fijian Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "PGK", Name: "Papua New Guinean Kina", Symbol: "K", DecimalPlaces: 2, Active: true},

		// Additional European Currencies
		{Code: "CZK", Name: "Czech Koruna", Symbol: "Kč", DecimalPlaces: 2, Active: true},
		{Code: "HUF", Name: "Hungarian Forint", Symbol: "Ft", DecimalPlaces: 2, Active: true},
		{Code: "RON", Name: "Romanian Leu", Symbol: "lei", DecimalPlaces: 2, Active: true},
		{Code: "UAH", Name: "Ukrainian Hryvnia", Symbol: "₴", DecimalPlaces: 2, Active: true},
		{Code: "BGN", Name: "Bulgarian Lev", Symbol: "лв", DecimalPlaces: 2, Active: true},
		{Code: "ISK", Name: "Icelandic Króna", Symbol: "kr", DecimalPlaces: 0, Active: true},
		{Code: "RSD", Name: "Serbian Dinar", Symbol: "дин.", DecimalPlaces: 2, Active: true},
		{Code: "BAM", Name: "Bosnia-Herzegovina Convertible Mark", Symbol: "KM", DecimalPlaces: 2, Active: true},
		{Code: "MKD", Name: "Macedonian Denar", Symbol: "ден", DecimalPlaces: 2, Active: true},
		{Code: "ALL", Name: "Albanian Lek", Symbol: "L", DecimalPlaces: 2, Active: true},
		{Code: "MDL", Name: "Moldovan Leu", Symbol: "L", DecimalPlaces: 2, Active: true},
		{Code: "BYN", Name: "Belarusian Ruble", Symbol: "Br", DecimalPlaces: 2, Active: true},

		// Central American Currencies
		{Code: "PAB", Name: "Panamanian Balboa", Symbol: "B/.", DecimalPlaces: 2, Active: true},
		{Code: "CRC", Name: "Costa Rican Colón", Symbol: "₡", DecimalPlaces: 2, Active: true},
		{Code: "GTQ", Name: "Guatemalan Quetzal", Symbol: "Q", DecimalPlaces: 2, Active: true},
		{Code: "HNL", Name: "Honduran Lempira", Symbol: "L", DecimalPlaces: 2, Active: true},
		{Code: "NIO", Name: "Nicaraguan Córdoba", Symbol: "C$", DecimalPlaces: 2, Active: true},
		{Code: "BZD", Name: "Belize Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},

		// Caribbean Currencies
		{Code: "CUP", Name: "Cuban Peso", Symbol: "₱", DecimalPlaces: 2, Active: true},
		{Code: "DOP", Name: "Dominican Peso", Symbol: "RD$", DecimalPlaces: 2, Active: true},
		{Code: "JMD", Name: "Jamaican Dollar", Symbol: "J$", DecimalPlaces: 2, Active: true},
		{Code: "HTG", Name: "Haitian Gourde", Symbol: "G", DecimalPlaces: 2, Active: true},
		{Code: "TTD", Name: "Trinidad and Tobago Dollar", Symbol: "TT$", DecimalPlaces: 2, Active: true},

		// Additional South American Currencies
		{Code: "VES", Name: "Venezuelan Bolívar", Symbol: "Bs.S", DecimalPlaces: 2, Active: true},
		{Code: "BOB", Name: "Bolivian Boliviano", Symbol: "Bs.", DecimalPlaces: 2, Active: true},
		{Code: "PYG", Name: "Paraguayan Guaraní", Symbol: "₲", DecimalPlaces: 0, Active: true},
		{Code: "UYU", Name: "Uruguayan Peso", Symbol: "$U", DecimalPlaces: 2, Active: true},
		{Code: "GYD", Name: "Guyanese Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "SRD", Name: "Surinamese Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},

		// Additional African Currencies
		{Code: "TZS", Name: "Tanzanian Shilling", Symbol: "TSh", DecimalPlaces: 2, Active: true},
		{Code: "ETB", Name: "Ethiopian Birr", Symbol: "Br", DecimalPlaces: 2, Active: true},
		{Code: "UGX", Name: "Ugandan Shilling", Symbol: "USh", DecimalPlaces: 0, Active: true},
		{Code: "RWF", Name: "Rwandan Franc", Symbol: "FRw", DecimalPlaces: 0, Active: true},
		{Code: "DZD", Name: "Algerian Dinar", Symbol: "د.ج", DecimalPlaces: 2, Active: true},
		{Code: "TND", Name: "Tunisian Dinar", Symbol: "د.ت", DecimalPlaces: 3, Active: true},
		{Code: "LYD", Name: "Libyan Dinar", Symbol: "ل.د", DecimalPlaces: 3, Active: true},
		{Code: "SDG", Name: "Sudanese Pound", Symbol: "ج.س.", DecimalPlaces: 2, Active: true},
		{Code: "AOA", Name: "Angolan Kwanza", Symbol: "Kz", DecimalPlaces: 2, Active: true},
		{Code: "MZN", Name: "Mozambican Metical", Symbol: "MT", DecimalPlaces: 2, Active: true},
		{Code: "ZWL", Name: "Zimbabwean Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "ZMW", Name: "Zambian Kwacha", Symbol: "ZK", DecimalPlaces: 2, Active: true},
		{Code: "BWP", Name: "Botswana Pula", Symbol: "P", DecimalPlaces: 2, Active: true},
		{Code: "NAD", Name: "Namibian Dollar", Symbol: "$", DecimalPlaces: 2, Active: true},
		{Code: "XOF", Name: "West African CFA Franc", Symbol: "CFA", DecimalPlaces: 0, Active: true},
		{Code: "XAF", Name: "Central African CFA Franc", Symbol: "FCFA", DecimalPlaces: 0, Active: true},
	}
}
