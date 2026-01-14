-- Add comprehensive settings fields to support all categories
ALTER TABLE settings ADD COLUMN IF NOT EXISTS ecommerce JSONB NOT NULL DEFAULT '{}';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS security JSONB NOT NULL DEFAULT '{}';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS notifications JSONB NOT NULL DEFAULT '{}';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS marketing JSONB NOT NULL DEFAULT '{}';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS integrations JSONB NOT NULL DEFAULT '{}';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS performance JSONB NOT NULL DEFAULT '{}';
ALTER TABLE settings ADD COLUMN IF NOT EXISTS compliance JSONB NOT NULL DEFAULT '{}';

-- Add featured column to settings_presets
ALTER TABLE settings_presets ADD COLUMN IF NOT EXISTS featured BOOLEAN NOT NULL DEFAULT false;

-- Update settings_presets to support the new comprehensive categories
ALTER TABLE settings_presets DROP CONSTRAINT IF EXISTS settings_presets_category_check;
ALTER TABLE settings_presets ADD CONSTRAINT settings_presets_category_check 
  CHECK (category IN ('theme', 'layout', 'ecommerce', 'security', 'notifications', 'marketing', 'integrations', 'performance', 'compliance', 'complete'));

-- Add comprehensive ecommerce preset with all 10 subcategories
INSERT INTO settings_presets (name, description, category, settings, is_default, tags, featured) 
VALUES 
(
    'Comprehensive Ecommerce Platform', 
    'Enterprise-grade ecommerce configuration with all 10 subcategories', 
    'ecommerce', 
    '{
      "ecommerce": {
        "store": {
          "name": "Your Store",
          "tagline": "Professional Online Store",
          "description": "Your trusted online retail destination",
          "contactEmail": "support@yourstore.com",
          "supportEmail": "help@yourstore.com",
          "supportPhone": "+1-800-SUPPORT",
          "address": {
            "businessName": "Your Business LLC",
            "street1": "123 Business Ave",
            "city": "Your City",
            "state": "CA",
            "zipCode": "90210",
            "country": "US"
          }
        },
        "catalog": {
          "categories": {
            "maxDepth": 5,
            "enableImages": true,
            "enableDescriptions": true,
            "displayType": "grid"
          },
          "products": {
            "skuFormat": "YS-{YYYY}-{####}",
            "enableVariants": true,
            "enableBundles": true,
            "enableDigitalProducts": true,
            "enableSubscriptions": true,
            "maxImages": 15,
            "enableReviews": true,
            "enableWishlist": true
          }
        },
        "inventory": {
          "tracking": {
            "enabled": true,
            "trackByVariant": true,
            "trackByLocation": true
          },
          "stockLevels": {
            "lowStockThreshold": 10,
            "criticalStockThreshold": 5,
            "autoReorderEnabled": false
          }
        },
        "pricing": {
          "display": {
            "showPrices": true,
            "requireLoginForPrices": false,
            "showCompareAtPrices": true,
            "priceFormat": "${amount}"
          },
          "tax": {
            "enabled": true,
            "calculation": "exclusive",
            "defaultRate": 8.25
          },
          "currencies": {
            "primary": "USD",
            "supported": ["USD", "EUR", "GBP"],
            "autoConversion": false
          }
        },
        "orders": {
          "numbering": {
            "format": "YS-{YYYY}-{####}",
            "prefix": "YS",
            "startingNumber": 10000
          },
          "processing": {
            "autoConfirm": true,
            "requireManualReview": false,
            "autoCapture": false
          },
          "limits": {
            "minimumOrderAmount": 0,
            "maximumOrderAmount": 10000
          }
        },
        "shipping": {
          "general": {
            "enabled": true,
            "enableLocalDelivery": false,
            "enableStorePickup": false,
            "enableInternational": false
          },
          "freeShipping": {
            "enabled": true,
            "threshold": 75,
            "applicableCountries": ["US"]
          }
        },
        "returns": {
          "policy": {
            "enabled": true,
            "period": 30,
            "requireOriginalPackaging": false,
            "allowPartialReturns": true
          },
          "costs": {
            "returnShippingPaidBy": "customer",
            "restockingFee": 0
          }
        },
        "checkout": {
          "cart": {
            "enablePersistentCart": true,
            "cartExpirationDays": 30,
            "enableCartRecovery": true
          },
          "flow": {
            "steps": ["cart", "information", "shipping", "payment", "review"],
            "showProgressIndicator": true
          }
        },
        "customers": {
          "accounts": {
            "enableRegistration": true,
            "requireEmailVerification": true,
            "allowGuestCheckout": true
          },
          "profiles": {
            "enableAddressBook": true,
            "enableWishlist": true,
            "enableOrderHistory": true
          }
        },
        "marketplace": {
          "enabled": false,
          "vendors": {
            "enableVendorRegistration": false,
            "requireVendorApproval": true
          }
        }
      }
    }',
    false,
    '["ecommerce", "comprehensive", "enterprise", "b2c", "modern"]',
    true
),
(
    'B2B Wholesale Platform', 
    'B2B-focused ecommerce configuration with wholesale features', 
    'ecommerce', 
    '{
      "ecommerce": {
        "pricing": {
          "display": {
            "showPrices": false,
            "requireLoginForPrices": true,
            "showCompareAtPrices": false
          },
          "discounts": {
            "enableBulkPricing": true,
            "enableTieredPricing": true
          }
        },
        "orders": {
          "limits": {
            "minimumOrderAmount": 100,
            "maximumOrderAmount": 50000
          },
          "checkout": {
            "requireAccountCreation": true,
            "collectPhoneNumber": true
          }
        },
        "customers": {
          "accounts": {
            "enableRegistration": false,
            "allowGuestCheckout": false
          }
        }
      }
    }',
    false,
    '["b2b", "wholesale", "business", "bulk"]',
    false
),
(
    'Marketplace Multi-Vendor', 
    'Multi-vendor marketplace configuration', 
    'ecommerce', 
    '{
      "ecommerce": {
        "marketplace": {
          "enabled": true,
          "vendors": {
            "enableVendorRegistration": true,
            "requireVendorApproval": true,
            "enableVendorProfiles": true,
            "enableVendorRatings": true,
            "maxProductsPerVendor": 1000
          },
          "commissions": {
            "defaultCommissionPercent": 15,
            "enableTieredCommissions": true,
            "minimumPayout": 100,
            "payoutSchedule": "weekly"
          }
        }
      }
    }',
    false,
    '["marketplace", "multi-vendor", "commission", "vendors"]',
    false
)
ON CONFLICT DO NOTHING;