package seeder

import (
	"log"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"location-service/internal/models"
)

// SeedDatabase seeds the database with initial data using upsert logic
// This allows adding missing data without failing on duplicates
func SeedDatabase(db *gorm.DB) error {
	// Always run seeding with upsert to ensure all reference data exists
	// This is safe to run multiple times and will add missing data

	// Seed countries
	log.Println("Seeding/updating countries...")
	if err := seedCountries(db); err != nil {
		log.Printf("Warning: Failed to seed countries: %v", err)
	}

	// Seed states
	log.Println("Seeding/updating states...")
	if err := seedStates(db); err != nil {
		log.Printf("Warning: Failed to seed states: %v", err)
	}

	// Seed currencies
	log.Println("Seeding/updating currencies...")
	if err := seedCurrencies(db); err != nil {
		log.Printf("Warning: Failed to seed currencies: %v", err)
	}

	// Seed timezones
	log.Println("Seeding/updating timezones...")
	if err := seedTimezones(db); err != nil {
		log.Printf("Warning: Failed to seed timezones: %v", err)
	}

	// Log final counts
	var countryCount, stateCount, currencyCount, timezoneCount int64
	db.Model(&models.Country{}).Count(&countryCount)
	db.Model(&models.State{}).Count(&stateCount)
	db.Model(&models.Currency{}).Count(&currencyCount)
	db.Model(&models.Timezone{}).Count(&timezoneCount)
	log.Printf("Seeding complete - Countries: %d, States: %d, Currencies: %d, Timezones: %d",
		countryCount, stateCount, currencyCount, timezoneCount)

	return nil
}

func seedCountries(db *gorm.DB) error {
	countries := getCountriesData()
	inserted, updated := 0, 0

	for _, country := range countries {
		// Use upsert: insert if not exists, update if exists
		result := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "native_name", "capital", "region", "subregion", "currency", "languages", "calling_code", "flag_emoji", "latitude", "longitude", "active"}),
		}).Create(&country)

		if result.Error != nil {
			log.Printf("Failed to upsert country %s: %v", country.Name, result.Error)
		} else if result.RowsAffected > 0 {
			inserted++
		} else {
			updated++
		}
	}

	log.Printf("Countries: %d inserted, %d updated", inserted, updated)
	return nil
}

func seedStates(db *gorm.DB) error {
	states := getStatesData()
	inserted, updated := 0, 0

	for _, state := range states {
		// Use upsert: insert if not exists, update if exists
		result := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "code", "country_id", "type", "latitude", "longitude", "active"}),
		}).Create(&state)

		if result.Error != nil {
			log.Printf("Failed to upsert state %s: %v", state.Name, result.Error)
		} else if result.RowsAffected > 0 {
			inserted++
		} else {
			updated++
		}
	}

	log.Printf("States: %d inserted, %d updated", inserted, updated)
	return nil
}

func seedCurrencies(db *gorm.DB) error {
	currencies := getCurrenciesData()
	inserted, updated := 0, 0

	for _, currency := range currencies {
		// Use upsert: insert if not exists, update if exists
		result := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "code"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "symbol", "decimal_places", "active"}),
		}).Create(&currency)

		if result.Error != nil {
			log.Printf("Failed to upsert currency %s: %v", currency.Name, result.Error)
		} else if result.RowsAffected > 0 {
			inserted++
		} else {
			updated++
		}
	}

	log.Printf("Currencies: %d inserted, %d updated", inserted, updated)
	return nil
}

func seedTimezones(db *gorm.DB) error {
	timezones := getTimezonesData()
	inserted, updated := 0, 0

	for _, timezone := range timezones {
		// Use upsert: insert if not exists, update if exists
		result := db.Clauses(clause.OnConflict{
			Columns:   []clause.Column{{Name: "id"}},
			DoUpdates: clause.AssignmentColumns([]string{"name", "abbreviation", "offset", "dst", "countries"}),
		}).Create(&timezone)

		if result.Error != nil {
			log.Printf("Failed to upsert timezone %s: %v", timezone.Name, result.Error)
		} else if result.RowsAffected > 0 {
			inserted++
		} else {
			updated++
		}
	}

	log.Printf("Timezones: %d inserted, %d updated", inserted, updated)
	return nil
}
