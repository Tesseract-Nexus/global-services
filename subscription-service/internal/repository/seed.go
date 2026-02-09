package repository

import (
	"log"

	"gorm.io/gorm"
	"subscription-service/internal/models"
)

func SeedPlans(db *gorm.DB) error {
	plans := []models.SubscriptionPlan{
		{
			Name:              "free",
			DisplayName:       "Free",
			Description:       "Get started with basic features",
			MonthlyPriceCents: 0,
			YearlyPriceCents:  0,
			Currency:          "usd",
			MaxProducts:       100,
			MaxUsers:          2,
			MaxStorageMB:      500,
			Features:          models.JSONB{"basic_analytics": true, "email_support": true},
			SortOrder:         0,
			IsActive:          true,
			IsFree:            true,
			TrialDays:         0,
		},
		{
			Name:              "starter",
			DisplayName:       "Starter",
			Description:       "Perfect for small businesses",
			MonthlyPriceCents: 2900,
			YearlyPriceCents:  29000,
			Currency:          "usd",
			MaxProducts:       1000,
			MaxUsers:          5,
			MaxStorageMB:      2048,
			Features:          models.JSONB{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true},
			SortOrder:         1,
			IsActive:          true,
			IsFree:            false,
			TrialDays:         14,
		},
		{
			Name:              "professional",
			DisplayName:       "Professional",
			Description:       "For growing businesses",
			MonthlyPriceCents: 7900,
			YearlyPriceCents:  79000,
			Currency:          "usd",
			MaxProducts:       10000,
			MaxUsers:          15,
			MaxStorageMB:      10240,
			Features:          models.JSONB{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true, "api_access": true, "custom_domain": true},
			SortOrder:         2,
			IsActive:          true,
			IsFree:            false,
			TrialDays:         14,
		},
		{
			Name:              "enterprise",
			DisplayName:       "Enterprise",
			Description:       "For large-scale operations",
			MonthlyPriceCents: 19900,
			YearlyPriceCents:  199000,
			Currency:          "usd",
			MaxProducts:       -1,
			MaxUsers:          -1,
			MaxStorageMB:      -1,
			Features:          models.JSONB{"basic_analytics": true, "advanced_analytics": true, "email_support": true, "priority_support": true, "api_access": true, "custom_domain": true, "dedicated_support": true, "sla": true},
			SortOrder:         3,
			IsActive:          true,
			IsFree:            false,
			TrialDays:         30,
		},
	}

	for _, plan := range plans {
		var existing models.SubscriptionPlan
		result := db.Where("name = ?", plan.Name).First(&existing)
		if result.Error != nil {
			if err := db.Create(&plan).Error; err != nil {
				return err
			}
			log.Printf("Seeded plan: %s", plan.DisplayName)
		}
	}

	return nil
}
