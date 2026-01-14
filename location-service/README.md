# Location Service

A comprehensive location data and geolocation service for the Tesseract Hub e-commerce platform. Provides country, state, currency, and timezone data with IP-based location detection.

## Features

### 🌍 Location Data Management
- Complete database of world countries with metadata
- States/provinces/regions for major countries
- Support for 70+ countries across all continents
- Native names, capitals, regions, and subregions
- Calling codes and flag emojis

### 💱 Currency Management
- 50+ world currencies with symbols
- Decimal place configuration
- Active/inactive status management
- Currency search and filtering

### 🕐 Timezone Support
- 70+ timezones worldwide
- DST (Daylight Saving Time) tracking
- UTC offset information
- Country-timezone mapping

### 📍 IP-Based Geolocation
- Automatic location detection from IP address
- Support for X-Forwarded-For headers
- Location caching for performance
- Fallback to mock data for development

### 🏠 Address Lookup & Autocomplete
- Real-time address autocomplete suggestions
- Multiple provider support (Google Places, Mapbox, Mock)
- Geocoding (address to coordinates)
- Reverse geocoding (coordinates to address)
- Address validation and standardization
- Manual address entry fallback when autocomplete fails
- Session token support for billing optimization

### 🔧 Admin CRUD Operations
- Full CRUD APIs for all entities
- Bulk upsert support
- Soft delete functionality
- Cache management endpoints

### 📊 Database Features
- PostgreSQL with GORM ORM
- Automatic migrations on startup
- Seed data for initial setup
- Schema versioning

## Quick Start with Docker

### Prerequisites
- Docker and Docker Compose installed
- Make (optional, for easier commands)

### 1. Clone and Setup
```bash
# Copy environment variables
cp .env.example .env

# Edit .env with your configuration
nano .env
```

### 2. Start Services
```bash
# Using Docker Compose
docker-compose up -d
```

### 3. Check Service Health
```bash
curl http://localhost:8085/health
```

The service will be available at:
- Location Service: `http://localhost:8085`
- PostgreSQL: `localhost:5432`

### 4. Stop Services
```bash
docker-compose down
```

## API Endpoints

### Location Detection
```http
GET /api/v1/location/detect              # Detect location from IP
GET /api/v1/location/detect?ip=8.8.8.8   # Detect for specific IP
```

### Countries
```http
GET /api/v1/countries                    # List all countries
GET /api/v1/countries?search=united      # Search countries
GET /api/v1/countries?region=Europe      # Filter by region
GET /api/v1/countries/:countryId         # Get country by ID
GET /api/v1/countries/:countryId/states  # Get states for country
```

### States
```http
GET /api/v1/states                       # List all states
GET /api/v1/states?country_id=US         # Filter by country
GET /api/v1/states?search=california     # Search states
GET /api/v1/states/:stateId              # Get state by ID
```

### Currencies
```http
GET /api/v1/currencies                   # List all currencies
GET /api/v1/currencies?active_only=true  # Only active currencies
GET /api/v1/currencies?search=dollar     # Search currencies
GET /api/v1/currencies/:currencyCode     # Get currency by code
```

### Timezones
```http
GET /api/v1/timezones                    # List all timezones
GET /api/v1/timezones?country_id=US      # Filter by country
GET /api/v1/timezones?search=pacific     # Search timezones
GET /api/v1/timezones/:timezoneId        # Get timezone by ID
```

### Address Lookup & Autocomplete
```http
# Autocomplete - Get address suggestions as user types
GET  /api/v1/address/autocomplete?input=123+main   # Autocomplete query
POST /api/v1/address/autocomplete                  # Autocomplete with options

# Geocoding - Convert address to coordinates
GET  /api/v1/address/geocode?address=123+Main+St   # Geocode address
POST /api/v1/address/geocode                       # Geocode with body

# Reverse Geocoding - Convert coordinates to address
GET  /api/v1/address/reverse-geocode?latitude=37.7749&longitude=-122.4194
POST /api/v1/address/reverse-geocode

# Place Details - Get full details from place ID
GET  /api/v1/address/place-details?place_id=xxx    # Get place details

# Address Validation - Validate and standardize address
GET  /api/v1/address/validate?address=123+Main+St  # Validate address
POST /api/v1/address/validate                      # Validate with body

# Manual Address Entry - For fallback when autocomplete fails
POST /api/v1/address/format-manual                 # Format manual address

# Parse Address - Extract components from free-form address
POST /api/v1/address/parse                         # Parse raw address
```

### Admin - Countries
```http
POST   /api/v1/admin/countries           # Create country
PUT    /api/v1/admin/countries/:id       # Update country
DELETE /api/v1/admin/countries/:id       # Delete country
```

### Admin - States
```http
POST   /api/v1/admin/states              # Create state
PUT    /api/v1/admin/states/:id          # Update state
DELETE /api/v1/admin/states/:id          # Delete state
```

### Admin - Currencies
```http
POST   /api/v1/admin/currencies          # Create currency
PUT    /api/v1/admin/currencies/:code    # Update currency
DELETE /api/v1/admin/currencies/:code    # Delete currency
```

### Admin - Timezones
```http
POST   /api/v1/admin/timezones           # Create timezone
PUT    /api/v1/admin/timezones/:id       # Update timezone
DELETE /api/v1/admin/timezones/:id       # Delete timezone
```

### Admin - Cache
```http
GET  /api/v1/admin/cache/stats           # Get cache statistics
POST /api/v1/admin/cache/cleanup         # Cleanup expired entries
```

### Health & Metrics
```http
GET /health                              # Health check
GET /ready                               # Readiness check
GET /metrics                             # Prometheus metrics
```

## Usage Examples

### 1. Detect User Location
```bash
curl http://localhost:8085/api/v1/location/detect \
  -H "X-Forwarded-For: 157.49.233.100"
```

Response:
```json
{
  "success": true,
  "message": "Location detected successfully",
  "data": {
    "ip": "157.49.233.100",
    "country": "IN",
    "country_name": "India",
    "calling_code": "+91",
    "flag_emoji": "🇮🇳",
    "state": "IN-MH",
    "state_name": "Maharashtra",
    "city": "Mumbai",
    "currency": "INR",
    "timezone": "Asia/Kolkata"
  }
}
```

### 2. List Countries with Pagination
```bash
curl "http://localhost:8085/api/v1/countries?limit=10&offset=0&region=Europe"
```

Response:
```json
{
  "success": true,
  "message": "Countries retrieved successfully",
  "data": [
    {
      "id": "DE",
      "name": "Germany",
      "native_name": "Deutschland",
      "capital": "Berlin",
      "region": "Europe",
      "currency": "EUR",
      "calling_code": "+49",
      "flag_emoji": "🇩🇪"
    }
  ],
  "pagination": {
    "total": 25,
    "limit": 10,
    "offset": 0,
    "has_next": true,
    "has_previous": false
  }
}
```

### 3. Get States for a Country
```bash
curl http://localhost:8085/api/v1/countries/US/states
```

### 4. Create a New Country (Admin)
```bash
curl -X POST http://localhost:8085/api/v1/admin/countries \
  -H "Content-Type: application/json" \
  -d '{
    "id": "XX",
    "name": "New Country",
    "native_name": "New Country",
    "capital": "Capital City",
    "region": "Europe",
    "currency": "EUR",
    "calling_code": "+99",
    "flag_emoji": "🏳️",
    "active": true
  }'
```

### 5. Get Cache Statistics
```bash
curl http://localhost:8085/api/v1/admin/cache/stats
```

Response:
```json
{
  "success": true,
  "data": {
    "total_entries": 1500,
    "expired_entries": 50,
    "valid_entries": 1450
  }
}
```

### 6. Address Autocomplete
```bash
curl "http://localhost:8085/api/v1/address/autocomplete?input=123+main"
```

Response:
```json
{
  "success": true,
  "message": "Address suggestions retrieved successfully",
  "data": {
    "suggestions": [
      {
        "place_id": "ChIJd8BlQ2BZwokRAFUEcm_qrcA",
        "description": "123 Main Street, New York, NY, USA",
        "main_text": "123 Main Street",
        "secondary_text": "New York, NY, USA",
        "types": ["street_address"]
      }
    ],
    "allow_manual_entry": true
  }
}
```

### 7. Geocode Address
```bash
curl "http://localhost:8085/api/v1/address/geocode?address=1600+Amphitheatre+Parkway+Mountain+View+CA"
```

Response:
```json
{
  "success": true,
  "message": "Address geocoded successfully",
  "data": {
    "formatted_address": "1600 Amphitheatre Pkwy, Mountain View, CA 94043, USA",
    "place_id": "ChIJ2eUgeAK6j4ARbn5u_wAGqWA",
    "location": {
      "latitude": 37.4224764,
      "longitude": -122.0842499
    },
    "components": [
      {"type": "street_number", "long_name": "1600", "short_name": "1600"},
      {"type": "route", "long_name": "Amphitheatre Parkway", "short_name": "Amphitheatre Pkwy"},
      {"type": "locality", "long_name": "Mountain View", "short_name": "Mountain View"},
      {"type": "administrative_area_level_1", "long_name": "California", "short_name": "CA"},
      {"type": "country", "long_name": "United States", "short_name": "US"},
      {"type": "postal_code", "long_name": "94043", "short_name": "94043"}
    ]
  }
}
```

### 8. Manual Address Entry (Fallback)
```bash
curl -X POST http://localhost:8085/api/v1/address/format-manual \
  -H "Content-Type: application/json" \
  -d '{
    "street_address": "123 Main Street",
    "apartment_unit": "Apt 4B",
    "city": "San Francisco",
    "state": "California",
    "postal_code": "94102",
    "country": "United States"
  }'
```

Response:
```json
{
  "success": true,
  "message": "Manual address formatted successfully",
  "data": {
    "formatted_address": "123 Main Street, Apt 4B, San Francisco, California 94102, United States",
    "components": [
      {"type": "subpremise", "long_name": "Apt 4B", "short_name": "Apt 4B"},
      {"type": "route", "long_name": "123 Main Street", "short_name": "123 Main Street"},
      {"type": "locality", "long_name": "San Francisco", "short_name": "San Francisco"},
      {"type": "administrative_area_level_1", "long_name": "California", "short_name": "California"},
      {"type": "postal_code", "long_name": "94102", "short_name": "94102"},
      {"type": "country", "long_name": "United States", "short_name": "United States"}
    ],
    "manual_entry": true
  }
}
```

### 9. Validate Address
```bash
curl "http://localhost:8085/api/v1/address/validate?address=123+Main+St+San+Francisco+CA"
```

Response:
```json
{
  "success": true,
  "message": "Address validation completed",
  "data": {
    "valid": true,
    "formatted_address": "123 Main St, San Francisco, CA 94102, USA",
    "components": [...],
    "location": {"latitude": 37.7749, "longitude": -122.4194},
    "deliverable": true,
    "issues": []
  }
}
```

## Data Coverage

### Countries (70+)
| Region | Countries |
|--------|-----------|
| Americas | US, CA, MX, BR, AR, CL, CO, PE, VE, EC, UY, PA, CR, PR, JM, DO |
| Europe | GB, DE, FR, IT, ES, NL, SE, CH, IE, AT, BE, CZ, DK, FI, GR, HU, NO, PL, PT, RO, RU, UA |
| Asia | IN, JP, CN, SG, KR, AE, IL, SA, TR, QA, KW, BH, OM, TH, VN, BD, PK, LK, NP, MY, ID, PH, HK, TW |
| Africa | ZA, EG, NG, KE, GH, MA, TN, TZ, UG, ET |
| Oceania | AU, NZ, FJ |

### States/Regions
- **United States**: All 50 states + DC
- **India**: All 36 states and union territories
- **Canada**: All 13 provinces and territories
- **Australia**: All 8 states and territories
- **United Kingdom**: 4 constituent countries
- **Germany**: All 16 Bundesländer
- **France**: All 13 régions
- **Japan**: 10 major prefectures
- **Brazil**: 10 major states
- **Mexico**: 10 major states
- **UAE**: All 7 emirates
- **Spain**: All 17 autonomous communities
- **Italy**: All 20 regions

### Currencies (50+)
USD, EUR, GBP, JPY, CAD, AUD, CHF, CNY, INR, SGD, NZD, ZAR, AED, BRL, MXN, KRW, SEK, PHP, MYR, IDR, CZK, DKK, HUF, NOK, PLN, TRY, THB, VND, PKR, and more...

### Timezones (70+)
All major world timezones including America/*, Europe/*, Asia/*, Africa/*, Australia/*, Pacific/*

## Environment Variables

```env
# Server Configuration
PORT=8085
GIN_MODE=debug

# Database Configuration
DB_HOST=localhost
DB_PORT=5432
DB_USER=dev
DB_PASSWORD=devpass
DB_NAME=location
DB_SSLMODE=disable

# Optional: External Geolocation API
GEO_PROVIDER=mock                    # mock, maxmind, ipapi
GEOLOCATION_API_KEY=your-api-key
GEOLOCATION_API_URL=https://api.ipgeolocation.io/ipgeo

# Address Lookup Provider Configuration
ADDRESS_PROVIDER=mapbox              # mock, google, mapbox, here
GOOGLE_MAPS_API_KEY=your-google-api-key
MAPBOX_ACCESS_TOKEN=your-mapbox-token
HERE_API_KEY=your-here-api-key
```

## Address Provider Setup Guide

The location service supports multiple address lookup providers. Choose based on your needs:

| Provider | Free Tier | Cost After Free | Best For |
|----------|-----------|-----------------|----------|
| **Mapbox** (Default) | 100k req/month | ~$0.75/1000 | Cost-effective, good accuracy |
| **Google Places** | $200 credit/month | ~$2.83/1000 | Best accuracy, especially India |
| **Here Maps** | 250k req/month | ~$1.00/1000 | High volume applications |
| **Mock** | Unlimited | Free | Development & testing |

### Option 1: Mapbox (Recommended)

**Step 1: Create Account**
1. Go to [mapbox.com](https://www.mapbox.com)
2. Click "Sign up" and create a free account
3. Verify your email address

**Step 2: Get Access Token**
1. Log in to your Mapbox account
2. Go to **Account** → **Access tokens** (or visit [account.mapbox.com/access-tokens](https://account.mapbox.com/access-tokens))
3. Copy your **Default public token** or create a new one
4. Token format: `pk.eyJ1Ijoi...` (starts with `pk.`)

**Step 3: Configure**
```bash
export ADDRESS_PROVIDER=mapbox
export MAPBOX_ACCESS_TOKEN=pk.eyJ1IjoieW91ci10b2tlbi1oZXJl...
```

### Option 2: Google Places API

**Step 1: Create Google Cloud Project**
1. Go to [Google Cloud Console](https://console.cloud.google.com)
2. Create a new project or select existing one
3. Enable billing (required, but $200 free credit/month)

**Step 2: Enable Required APIs**
1. Go to **APIs & Services** → **Library**
2. Search and enable these APIs:
   - **Places API** (for autocomplete)
   - **Geocoding API** (for address-to-coordinates)
   - **Maps JavaScript API** (optional, for frontend maps)

**Step 3: Create API Key**
1. Go to **APIs & Services** → **Credentials**
2. Click **Create Credentials** → **API Key**
3. Click **Edit API Key** to add restrictions:
   - **Application restrictions**: HTTP referrers or IP addresses
   - **API restrictions**: Select only Places API and Geocoding API
4. Copy your API key
5. Key format: `AIzaSy...` (starts with `AIza`)

**Step 4: Configure**
```bash
export ADDRESS_PROVIDER=google
export GOOGLE_MAPS_API_KEY=AIzaSyB-your-api-key-here
```

### Option 3: Here Maps

**Step 1: Create Account**
1. Go to [developer.here.com](https://developer.here.com)
2. Click "Sign up" for a free account
3. Verify your email and complete registration

**Step 2: Create Project & Get API Key**
1. Log in to the [HERE Developer Portal](https://developer.here.com)
2. Go to **Projects** → **Create new project**
3. Name your project (e.g., "Tesseract Location Service")
4. Go to your project → **REST** → **Generate App**
5. Create new API key
6. Copy the API Key
7. Key format: Long alphanumeric string

**Step 3: Configure**
```bash
export ADDRESS_PROVIDER=here
export HERE_API_KEY=your-here-api-key-here
```

### Option 4: Mock Provider (Development)

For local development and testing, use the mock provider:
```bash
export ADDRESS_PROVIDER=mock
# No API key needed - returns sample addresses
```

## Kubernetes Sealed Secrets

For production deployments, use Kubernetes SealedSecrets to securely store API keys.

### Generate Sealed Secret

```bash
# Set your kubeconfig
export KUBECONFIG=~/.kube/your-cluster

# For Mapbox
echo -n 'pk.eyJ1IjoieW91ci10b2tlbi1oZXJl...' | kubeseal --raw \
  --namespace your-namespace \
  --name location-service-address-secrets \
  --controller-name sealed-secrets \
  --controller-namespace kube-system \
  --from-file=/dev/stdin

# For Google Places
echo -n 'AIzaSyB-your-api-key-here' | kubeseal --raw \
  --namespace your-namespace \
  --name location-service-google-secrets \
  --controller-name sealed-secrets \
  --controller-namespace kube-system \
  --from-file=/dev/stdin

# For Here Maps
echo -n 'your-here-api-key-here' | kubeseal --raw \
  --namespace your-namespace \
  --name location-service-here-secrets \
  --controller-name sealed-secrets \
  --controller-namespace kube-system \
  --from-file=/dev/stdin
```

### Helm Values Configuration

```yaml
# values.yaml
env:
  ADDRESS_PROVIDER: "mapbox"  # or "google", "here", "mock"

# Enable the provider you want to use
addressProvider:
  mapbox:
    enabled: true
    existingSecret: "location-service-address-secrets"
    secretKey: "MAPBOX_ACCESS_TOKEN"
  google:
    enabled: false
    existingSecret: "location-service-google-secrets"
    secretKey: "GOOGLE_MAPS_API_KEY"
  here:
    enabled: false
    existingSecret: "location-service-here-secrets"
    secretKey: "HERE_API_KEY"

# Add encrypted data for enabled providers
addressSealedSecrets:
  mapbox:
    enabled: true
    name: "location-service-address-secrets"
    encryptedData:
      MAPBOX_ACCESS_TOKEN: "AgC0uVBD..."  # Output from kubeseal
  google:
    enabled: false
    name: "location-service-google-secrets"
    encryptedData: {}
  here:
    enabled: false
    name: "location-service-here-secrets"
    encryptedData: {}
```

### Switching Providers

To switch from Mapbox to Google:

1. Get your Google API key and generate sealed secret
2. Update `values.yaml`:
```yaml
env:
  ADDRESS_PROVIDER: "google"

addressProvider:
  mapbox:
    enabled: false
  google:
    enabled: true

addressSealedSecrets:
  mapbox:
    enabled: false
  google:
    enabled: true
    encryptedData:
      GOOGLE_MAPS_API_KEY: "AgBY..."  # Your sealed value
```
3. Deploy the updated configuration

## Development Setup

### 1. Prerequisites
- Go 1.21+
- PostgreSQL 13+

### 2. Database Setup
```bash
# Create database
createdb location

# Migrations run automatically on startup
```

### 3. Run Locally
```bash
# Install dependencies
go mod download

# Set environment variables
export DB_HOST=localhost
export DB_NAME=location
export DB_USER=dev
export DB_PASSWORD=devpass

# Run the service
go run cmd/main.go
```

### 4. Run Tests
```bash
go test ./...
```

## Database Schema

### Tables

#### countries
| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR(2) | ISO 3166-1 alpha-2 code (PK) |
| name | VARCHAR(100) | Country name |
| native_name | VARCHAR(100) | Native language name |
| capital | VARCHAR(100) | Capital city |
| region | VARCHAR(50) | Continent/region |
| subregion | VARCHAR(100) | Subregion |
| currency | VARCHAR(3) | Default currency code |
| languages | JSONB | Supported languages |
| calling_code | VARCHAR(10) | Phone calling code |
| flag_emoji | VARCHAR(10) | Flag emoji |
| latitude | DECIMAL | Center latitude |
| longitude | DECIMAL | Center longitude |
| active | BOOLEAN | Is active |

#### states
| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR(10) | State ID (e.g., US-CA) (PK) |
| name | VARCHAR(100) | State name |
| native_name | VARCHAR(100) | Native name |
| code | VARCHAR(10) | State code |
| country_id | VARCHAR(2) | Country reference (FK) |
| type | VARCHAR(50) | Type (state, province, etc.) |
| latitude | DECIMAL | Center latitude |
| longitude | DECIMAL | Center longitude |
| active | BOOLEAN | Is active |

#### currencies
| Column | Type | Description |
|--------|------|-------------|
| code | VARCHAR(3) | ISO 4217 code (PK) |
| name | VARCHAR(100) | Currency name |
| symbol | VARCHAR(10) | Currency symbol |
| decimal_places | INTEGER | Decimal places |
| active | BOOLEAN | Is active |

#### timezones
| Column | Type | Description |
|--------|------|-------------|
| id | VARCHAR(50) | IANA timezone ID (PK) |
| name | VARCHAR(100) | Display name |
| abbreviation | VARCHAR(20) | Timezone abbreviation |
| offset | VARCHAR(10) | UTC offset |
| dst | BOOLEAN | Has DST |
| countries | JSONB | Associated countries |

#### location_cache
| Column | Type | Description |
|--------|------|-------------|
| id | UUID | Cache entry ID (PK) |
| ip | VARCHAR(50) | IP address (unique) |
| country_id | VARCHAR(2) | Detected country |
| state_id | VARCHAR(10) | Detected state |
| timezone_id | VARCHAR(50) | Detected timezone |
| expires_at | TIMESTAMP | Cache expiration |

## Migrations

Migrations run automatically on service startup. Files are located in:
```
internal/migration/sql/
├── 000001_init_schema.up.sql      # Schema creation
├── 000001_init_schema.down.sql    # Schema rollback
├── 000002_seed_data.up.sql        # Initial seed data
├── 000002_seed_data.down.sql      # Seed rollback
├── 000003_expand_world_data.up.sql    # Extended world data
└── 000003_expand_world_data.down.sql  # Extended data rollback
```

## Monitoring & Observability

### Health Check
```bash
curl http://localhost:8085/health
```

### Readiness Check
```bash
curl http://localhost:8085/ready
```

### Metrics (Prometheus format)
```bash
curl http://localhost:8085/metrics
```

Available metrics:
- `tesseract_location_detections_total` - Location detection requests
- `tesseract_location_country_lookups_total` - Country lookup requests
- `tesseract_location_state_lookups_total` - State lookup requests
- `tesseract_location_cache_hits_total` - Cache hits
- `tesseract_location_cache_misses_total` - Cache misses
- `tesseract_location_db_connections_open` - Open DB connections
- `tesseract_location_cached_locations` - Cached location count

## Architecture

```
location-service/
├── cmd/
│   └── main.go                 # Application entry point
├── internal/
│   ├── config/                 # Configuration management
│   ├── handlers/               # HTTP handlers
│   ├── middleware/             # HTTP middleware (CORS, etc.)
│   ├── migration/              # Database migrations
│   │   └── sql/               # SQL migration files
│   ├── models/                 # Data models
│   ├── repository/             # Database repositories
│   │   ├── country_repository.go
│   │   ├── state_repository.go
│   │   ├── currency_repository.go
│   │   ├── timezone_repository.go
│   │   └── location_cache_repository.go
│   ├── seeder/                 # Data seeders
│   └── services/               # Business logic
│       ├── location_service.go
│       ├── geolocation_service.go
│       └── address_service.go    # Address lookup providers
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── README.md
```

## Integration with Frontend

### Auto-fill Location Data
```typescript
// Detect user location
const detectLocation = async () => {
  const response = await fetch('/api/v1/location/detect');
  const { data } = await response.json();

  return {
    country: data.country,
    countryName: data.country_name,
    callingCode: data.calling_code,
    state: data.state,
    stateName: data.state_name,
    city: data.city,
    currency: data.currency,
    timezone: data.timezone
  };
};

// Auto-fill form fields
const location = await detectLocation();
form.setValue('country', location.country);
form.setValue('state', location.state);
form.setValue('phoneCountryCode', location.callingCode);
form.setValue('currency', location.currency);
form.setValue('timezone', location.timezone);
```

### Load Dynamic Dropdowns
```typescript
// Load countries
const countries = await fetch('/api/v1/countries').then(r => r.json());

// Load states for selected country
const states = await fetch(`/api/v1/countries/${countryId}/states`).then(r => r.json());

// Load currencies
const currencies = await fetch('/api/v1/currencies').then(r => r.json());

// Load timezones
const timezones = await fetch('/api/v1/timezones').then(r => r.json());
```

### Address Autocomplete with Fallback
```typescript
// Address autocomplete with manual entry fallback
const addressLookup = async (input: string, sessionToken: string) => {
  try {
    const response = await fetch(
      `/api/v1/address/autocomplete?input=${encodeURIComponent(input)}&session_token=${sessionToken}`
    );
    const { data } = await response.json();

    return {
      suggestions: data.suggestions,
      allowManualEntry: data.allow_manual_entry
    };
  } catch (error) {
    // On error, allow manual entry
    return { suggestions: [], allowManualEntry: true };
  }
};

// Get full address details when user selects a suggestion
const getAddressDetails = async (placeId: string) => {
  const response = await fetch(`/api/v1/address/place-details?place_id=${placeId}`);
  const { data } = await response.json();

  return {
    formattedAddress: data.formatted_address,
    streetNumber: data.components.find(c => c.type === 'street_number')?.long_name,
    route: data.components.find(c => c.type === 'route')?.long_name,
    city: data.components.find(c => c.type === 'locality')?.long_name,
    state: data.components.find(c => c.type === 'administrative_area_level_1')?.short_name,
    country: data.components.find(c => c.type === 'country')?.short_name,
    postalCode: data.components.find(c => c.type === 'postal_code')?.long_name,
    location: data.location
  };
};

// Manual address entry (fallback when autocomplete fails)
const formatManualAddress = async (address: ManualAddress) => {
  const response = await fetch('/api/v1/address/format-manual', {
    method: 'POST',
    headers: { 'Content-Type': 'application/json' },
    body: JSON.stringify({
      street_address: address.streetAddress,
      apartment_unit: address.apartmentUnit,
      city: address.city,
      state: address.state,
      postal_code: address.postalCode,
      country: address.country
    })
  });
  return response.json();
};

// Example usage in a form component
const AddressForm = () => {
  const [suggestions, setSuggestions] = useState([]);
  const [showManualEntry, setShowManualEntry] = useState(false);
  const sessionToken = useRef(crypto.randomUUID());

  const handleInputChange = async (value: string) => {
    if (value.length < 3) return;

    const result = await addressLookup(value, sessionToken.current);
    setSuggestions(result.suggestions);

    // Show manual entry if no suggestions found
    if (result.suggestions.length === 0 && result.allowManualEntry) {
      setShowManualEntry(true);
    }
  };

  // ... render form with suggestions or manual entry fields
};
```

## Production Deployment

### Docker
```bash
# Build image
docker build -t tesseract-location-service .

# Run container
docker run -p 8085:8085 \
  -e DB_HOST=postgres \
  -e DB_NAME=location \
  tesseract-location-service
```

### Kubernetes
```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: location-service
spec:
  replicas: 2
  selector:
    matchLabels:
      app: location-service
  template:
    spec:
      containers:
      - name: location-service
        image: tesseract-location-service:latest
        ports:
        - containerPort: 8085
        env:
        - name: DB_HOST
          valueFrom:
            configMapKeyRef:
              name: location-config
              key: db-host
        livenessProbe:
          httpGet:
            path: /health
            port: 8085
        readinessProbe:
          httpGet:
            path: /ready
            port: 8085
```

## Performance

- **Throughput**: ~10,000 requests/second (cached lookups)
- **Latency**: <10ms (cached), <50ms (database)
- **Memory**: ~50MB base usage
- **Cache TTL**: 24 hours (configurable)

## Mock Mode

When running without a database connection, the service operates in mock mode with in-memory data. This is useful for:
- Local development
- Testing
- Demo environments

Mock mode includes sample data for major countries, states, currencies, and timezones.

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## License

MIT License - see LICENSE file for details.
