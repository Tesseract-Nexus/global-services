package seeder

import (
	"log"

	"location-service/internal/models"
	"gorm.io/gorm"
)

// SeedDatabase seeds the database with initial data
func SeedDatabase(db *gorm.DB) error {
	// Check if countries already exist
	var count int64
	if err := db.Model(&models.Country{}).Count(&count).Error; err != nil {
		return err
	}

	if count > 0 {
		log.Printf("Database already seeded with %d countries, skipping...", count)
		return nil
	}

	// Seed countries
	log.Println("Seeding countries...")
	if err := seedCountries(db); err != nil {
		return err
	}

	// Seed states
	log.Println("Seeding states...")
	if err := seedStates(db); err != nil {
		return err
	}

	// Seed currencies
	log.Println("Seeding currencies...")
	if err := seedCurrencies(db); err != nil {
		return err
	}

	// Seed timezones
	log.Println("Seeding timezones...")
	if err := seedTimezones(db); err != nil {
		return err
	}

	return nil
}

func seedCountries(db *gorm.DB) error {
	countries := getCountriesData()

	for _, country := range countries {
		if err := db.Create(&country).Error; err != nil {
			log.Printf("Failed to seed country %s: %v", country.Name, err)
			// Continue with other countries even if one fails
		}
	}

	log.Printf("Seeded %d countries", len(countries))
	return nil
}

func seedStates(db *gorm.DB) error {
	states := getStatesData()

	for _, state := range states {
		if err := db.Create(&state).Error; err != nil {
			log.Printf("Failed to seed state %s: %v", state.Name, err)
			// Continue with other states even if one fails
		}
	}

	log.Printf("Seeded %d states", len(states))
	return nil
}

func seedCurrencies(db *gorm.DB) error {
	currencies := getCurrenciesData()

	for _, currency := range currencies {
		if err := db.Create(&currency).Error; err != nil {
			log.Printf("Failed to seed currency %s: %v", currency.Name, err)
			// Continue with other currencies even if one fails
		}
	}

	log.Printf("Seeded %d currencies", len(currencies))
	return nil
}

func seedTimezones(db *gorm.DB) error {
	timezones := getTimezonesData()

	for _, timezone := range timezones {
		if err := db.Create(&timezone).Error; err != nil {
			log.Printf("Failed to seed timezone %s: %v", timezone.Name, err)
			// Continue with other timezones even if one fails
		}
	}

	log.Printf("Seeded %d timezones", len(timezones))
	return nil
}
