-- Migration: 000002_seed_data
-- Description: Seed initial location data (countries, states, currencies, timezones)
-- Created: 2024-01-01

SET search_path TO location, public;

-- =====================================================
-- SEED COUNTRIES
-- =====================================================
INSERT INTO countries (id, name, native_name, capital, region, subregion, currency, languages, calling_code, flag_emoji, latitude, longitude, active)
VALUES
    ('US', 'United States', 'United States', 'Washington', 'Americas', 'Northern America', 'USD', '["en"]', '+1', '🇺🇸', 37.0902, -95.7129, true),
    ('CA', 'Canada', 'Canada', 'Ottawa', 'Americas', 'Northern America', 'CAD', '["en","fr"]', '+1', '🇨🇦', 56.1304, -106.3468, true),
    ('GB', 'United Kingdom', 'United Kingdom', 'London', 'Europe', 'Northern Europe', 'GBP', '["en"]', '+44', '🇬🇧', 55.3781, -3.4360, true),
    ('IN', 'India', 'भारत', 'New Delhi', 'Asia', 'Southern Asia', 'INR', '["hi","en"]', '+91', '🇮🇳', 20.5937, 78.9629, true),
    ('AU', 'Australia', 'Australia', 'Canberra', 'Oceania', 'Australia and New Zealand', 'AUD', '["en"]', '+61', '🇦🇺', -25.2744, 133.7751, true),
    ('DE', 'Germany', 'Deutschland', 'Berlin', 'Europe', 'Western Europe', 'EUR', '["de"]', '+49', '🇩🇪', 51.1657, 10.4515, true),
    ('FR', 'France', 'France', 'Paris', 'Europe', 'Western Europe', 'EUR', '["fr"]', '+33', '🇫🇷', 46.2276, 2.2137, true),
    ('JP', 'Japan', '日本', 'Tokyo', 'Asia', 'Eastern Asia', 'JPY', '["ja"]', '+81', '🇯🇵', 36.2048, 138.2529, true),
    ('CN', 'China', '中国', 'Beijing', 'Asia', 'Eastern Asia', 'CNY', '["zh"]', '+86', '🇨🇳', 35.8617, 104.1954, true),
    ('BR', 'Brazil', 'Brasil', 'Brasília', 'Americas', 'South America', 'BRL', '["pt"]', '+55', '🇧🇷', -14.2350, -51.9253, true),
    ('MX', 'Mexico', 'México', 'Mexico City', 'Americas', 'Central America', 'MXN', '["es"]', '+52', '🇲🇽', 23.6345, -102.5528, true),
    ('IT', 'Italy', 'Italia', 'Rome', 'Europe', 'Southern Europe', 'EUR', '["it"]', '+39', '🇮🇹', 41.8719, 12.5674, true),
    ('ES', 'Spain', 'España', 'Madrid', 'Europe', 'Southern Europe', 'EUR', '["es"]', '+34', '🇪🇸', 40.4637, -3.7492, true),
    ('NL', 'Netherlands', 'Nederland', 'Amsterdam', 'Europe', 'Western Europe', 'EUR', '["nl"]', '+31', '🇳🇱', 52.1326, 5.2913, true),
    ('SE', 'Sweden', 'Sverige', 'Stockholm', 'Europe', 'Northern Europe', 'SEK', '["sv"]', '+46', '🇸🇪', 60.1282, 18.6435, true),
    ('SG', 'Singapore', 'Singapore', 'Singapore', 'Asia', 'South-Eastern Asia', 'SGD', '["en","ms","zh","ta"]', '+65', '🇸🇬', 1.3521, 103.8198, true),
    ('NZ', 'New Zealand', 'New Zealand', 'Wellington', 'Oceania', 'Australia and New Zealand', 'NZD', '["en","mi"]', '+64', '🇳🇿', -40.9006, 174.8860, true),
    ('ZA', 'South Africa', 'South Africa', 'Pretoria', 'Africa', 'Southern Africa', 'ZAR', '["en","af","zu","xh"]', '+27', '🇿🇦', -30.5595, 22.9375, true),
    ('AE', 'United Arab Emirates', 'الإمارات العربية المتحدة', 'Abu Dhabi', 'Asia', 'Western Asia', 'AED', '["ar"]', '+971', '🇦🇪', 23.4241, 53.8478, true),
    ('KR', 'South Korea', '대한민국', 'Seoul', 'Asia', 'Eastern Asia', 'KRW', '["ko"]', '+82', '🇰🇷', 35.9078, 127.7669, true),
    ('CH', 'Switzerland', 'Schweiz', 'Bern', 'Europe', 'Western Europe', 'CHF', '["de","fr","it"]', '+41', '🇨🇭', 46.8182, 8.2275, true),
    ('IE', 'Ireland', 'Ireland', 'Dublin', 'Europe', 'Northern Europe', 'EUR', '["en","ga"]', '+353', '🇮🇪', 53.1424, -7.6921, true),
    ('PH', 'Philippines', 'Pilipinas', 'Manila', 'Asia', 'South-Eastern Asia', 'PHP', '["en","tl"]', '+63', '🇵🇭', 12.8797, 121.7740, true),
    ('MY', 'Malaysia', 'Malaysia', 'Kuala Lumpur', 'Asia', 'South-Eastern Asia', 'MYR', '["ms","en"]', '+60', '🇲🇾', 4.2105, 101.9758, true),
    ('ID', 'Indonesia', 'Indonesia', 'Jakarta', 'Asia', 'South-Eastern Asia', 'IDR', '["id"]', '+62', '🇮🇩', -0.7893, 113.9213, true)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    native_name = EXCLUDED.native_name,
    capital = EXCLUDED.capital,
    region = EXCLUDED.region,
    subregion = EXCLUDED.subregion,
    currency = EXCLUDED.currency,
    languages = EXCLUDED.languages,
    calling_code = EXCLUDED.calling_code,
    flag_emoji = EXCLUDED.flag_emoji,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED CURRENCIES
-- =====================================================
INSERT INTO currencies (code, name, symbol, decimal_places, active)
VALUES
    ('USD', 'United States Dollar', '$', 2, true),
    ('EUR', 'Euro', '€', 2, true),
    ('GBP', 'British Pound', '£', 2, true),
    ('JPY', 'Japanese Yen', '¥', 0, true),
    ('CAD', 'Canadian Dollar', 'C$', 2, true),
    ('AUD', 'Australian Dollar', 'A$', 2, true),
    ('CHF', 'Swiss Franc', 'CHF', 2, true),
    ('CNY', 'Chinese Yuan', '¥', 2, true),
    ('INR', 'Indian Rupee', '₹', 2, true),
    ('SGD', 'Singapore Dollar', 'S$', 2, true),
    ('NZD', 'New Zealand Dollar', 'NZ$', 2, true),
    ('ZAR', 'South African Rand', 'R', 2, true),
    ('AED', 'UAE Dirham', 'د.إ', 2, true),
    ('BRL', 'Brazilian Real', 'R$', 2, true),
    ('MXN', 'Mexican Peso', '$', 2, true),
    ('KRW', 'South Korean Won', '₩', 0, true),
    ('SEK', 'Swedish Krona', 'kr', 2, true),
    ('PHP', 'Philippine Peso', '₱', 2, true),
    ('MYR', 'Malaysian Ringgit', 'RM', 2, true),
    ('IDR', 'Indonesian Rupiah', 'Rp', 0, true)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    symbol = EXCLUDED.symbol,
    decimal_places = EXCLUDED.decimal_places,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED TIMEZONES
-- =====================================================
INSERT INTO timezones (id, name, abbreviation, "offset", dst, countries)
VALUES
    -- Americas
    ('America/New_York', 'Eastern Time', 'EST/EDT', '-05:00', true, '["US","CA"]'),
    ('America/Chicago', 'Central Time', 'CST/CDT', '-06:00', true, '["US","CA","MX"]'),
    ('America/Denver', 'Mountain Time', 'MST/MDT', '-07:00', true, '["US","CA","MX"]'),
    ('America/Los_Angeles', 'Pacific Time', 'PST/PDT', '-08:00', true, '["US","CA"]'),
    ('America/Anchorage', 'Alaska Time', 'AKST/AKDT', '-09:00', true, '["US"]'),
    ('Pacific/Honolulu', 'Hawaii-Aleutian Time', 'HST', '-10:00', false, '["US"]'),
    ('America/Toronto', 'Toronto Time', 'EST/EDT', '-05:00', true, '["CA"]'),
    ('America/Vancouver', 'Vancouver Time', 'PST/PDT', '-08:00', true, '["CA"]'),
    ('America/Sao_Paulo', 'Brasília Time', 'BRT/BRST', '-03:00', true, '["BR"]'),
    ('America/Mexico_City', 'Central Standard Time', 'CST', '-06:00', false, '["MX"]'),
    -- Europe
    ('Europe/London', 'Greenwich Mean Time', 'GMT/BST', '+00:00', true, '["GB","IE"]'),
    ('Europe/Paris', 'Central European Time', 'CET/CEST', '+01:00', true, '["FR","DE","IT","ES","NL"]'),
    ('Europe/Berlin', 'Central European Time', 'CET/CEST', '+01:00', true, '["DE"]'),
    ('Europe/Madrid', 'Central European Time', 'CET/CEST', '+01:00', true, '["ES"]'),
    ('Europe/Rome', 'Central European Time', 'CET/CEST', '+01:00', true, '["IT"]'),
    ('Europe/Amsterdam', 'Central European Time', 'CET/CEST', '+01:00', true, '["NL"]'),
    ('Europe/Stockholm', 'Central European Time', 'CET/CEST', '+01:00', true, '["SE"]'),
    ('Europe/Zurich', 'Central European Time', 'CET/CEST', '+01:00', true, '["CH"]'),
    ('Europe/Dublin', 'Greenwich Mean Time', 'GMT/IST', '+00:00', true, '["IE"]'),
    -- Asia
    ('Asia/Dubai', 'Gulf Standard Time', 'GST', '+04:00', false, '["AE"]'),
    ('Asia/Kolkata', 'India Standard Time', 'IST', '+05:30', false, '["IN"]'),
    ('Asia/Singapore', 'Singapore Time', 'SGT', '+08:00', false, '["SG","MY"]'),
    ('Asia/Tokyo', 'Japan Standard Time', 'JST', '+09:00', false, '["JP"]'),
    ('Asia/Seoul', 'Korea Standard Time', 'KST', '+09:00', false, '["KR"]'),
    ('Asia/Shanghai', 'China Standard Time', 'CST', '+08:00', false, '["CN"]'),
    ('Asia/Hong_Kong', 'Hong Kong Time', 'HKT', '+08:00', false, '["HK"]'),
    ('Asia/Manila', 'Philippine Time', 'PHT', '+08:00', false, '["PH"]'),
    ('Asia/Kuala_Lumpur', 'Malaysia Time', 'MYT', '+08:00', false, '["MY"]'),
    ('Asia/Jakarta', 'Western Indonesia Time', 'WIB', '+07:00', false, '["ID"]'),
    -- Oceania
    ('Australia/Sydney', 'Australian Eastern Time', 'AEST/AEDT', '+10:00', true, '["AU"]'),
    ('Australia/Melbourne', 'Australian Eastern Time', 'AEST/AEDT', '+10:00', true, '["AU"]'),
    ('Australia/Brisbane', 'Australian Eastern Standard Time', 'AEST', '+10:00', false, '["AU"]'),
    ('Australia/Perth', 'Australian Western Standard Time', 'AWST', '+08:00', false, '["AU"]'),
    ('Pacific/Auckland', 'New Zealand Time', 'NZST/NZDT', '+12:00', true, '["NZ"]'),
    -- Africa
    ('Africa/Johannesburg', 'South Africa Standard Time', 'SAST', '+02:00', false, '["ZA"]')
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    abbreviation = EXCLUDED.abbreviation,
    "offset" = EXCLUDED."offset",
    dst = EXCLUDED.dst,
    countries = EXCLUDED.countries,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED STATES - US STATES
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('US-AL', 'Alabama', 'Alabama', 'AL', 'US', 'state', 32.3182, -86.9023, true),
    ('US-AK', 'Alaska', 'Alaska', 'AK', 'US', 'state', 64.2008, -149.4937, true),
    ('US-AZ', 'Arizona', 'Arizona', 'AZ', 'US', 'state', 34.0489, -111.0937, true),
    ('US-AR', 'Arkansas', 'Arkansas', 'AR', 'US', 'state', 35.2010, -91.8318, true),
    ('US-CA', 'California', 'California', 'CA', 'US', 'state', 36.7783, -119.4179, true),
    ('US-CO', 'Colorado', 'Colorado', 'CO', 'US', 'state', 39.5501, -105.7821, true),
    ('US-CT', 'Connecticut', 'Connecticut', 'CT', 'US', 'state', 41.6032, -73.0877, true),
    ('US-DE', 'Delaware', 'Delaware', 'DE', 'US', 'state', 38.9108, -75.5277, true),
    ('US-FL', 'Florida', 'Florida', 'FL', 'US', 'state', 27.6648, -81.5158, true),
    ('US-GA', 'Georgia', 'Georgia', 'GA', 'US', 'state', 32.1656, -82.9001, true),
    ('US-HI', 'Hawaii', 'Hawaii', 'HI', 'US', 'state', 19.8968, -155.5828, true),
    ('US-ID', 'Idaho', 'Idaho', 'ID', 'US', 'state', 44.0682, -114.7420, true),
    ('US-IL', 'Illinois', 'Illinois', 'IL', 'US', 'state', 40.6331, -89.3985, true),
    ('US-IN', 'Indiana', 'Indiana', 'IN', 'US', 'state', 40.2672, -86.1349, true),
    ('US-IA', 'Iowa', 'Iowa', 'IA', 'US', 'state', 41.8780, -93.0977, true),
    ('US-KS', 'Kansas', 'Kansas', 'KS', 'US', 'state', 39.0119, -98.4842, true),
    ('US-KY', 'Kentucky', 'Kentucky', 'KY', 'US', 'state', 37.8393, -84.2700, true),
    ('US-LA', 'Louisiana', 'Louisiana', 'LA', 'US', 'state', 30.9843, -91.9623, true),
    ('US-ME', 'Maine', 'Maine', 'ME', 'US', 'state', 45.2538, -69.4455, true),
    ('US-MD', 'Maryland', 'Maryland', 'MD', 'US', 'state', 39.0458, -76.6413, true),
    ('US-MA', 'Massachusetts', 'Massachusetts', 'MA', 'US', 'state', 42.4072, -71.3824, true),
    ('US-MI', 'Michigan', 'Michigan', 'MI', 'US', 'state', 44.3148, -85.6024, true),
    ('US-MN', 'Minnesota', 'Minnesota', 'MN', 'US', 'state', 46.7296, -94.6859, true),
    ('US-MS', 'Mississippi', 'Mississippi', 'MS', 'US', 'state', 32.3547, -89.3985, true),
    ('US-MO', 'Missouri', 'Missouri', 'MO', 'US', 'state', 37.9643, -91.8318, true),
    ('US-MT', 'Montana', 'Montana', 'MT', 'US', 'state', 46.8797, -110.3626, true),
    ('US-NE', 'Nebraska', 'Nebraska', 'NE', 'US', 'state', 41.4925, -99.9018, true),
    ('US-NV', 'Nevada', 'Nevada', 'NV', 'US', 'state', 38.8026, -116.4194, true),
    ('US-NH', 'New Hampshire', 'New Hampshire', 'NH', 'US', 'state', 43.1939, -71.5724, true),
    ('US-NJ', 'New Jersey', 'New Jersey', 'NJ', 'US', 'state', 40.0583, -74.4057, true),
    ('US-NM', 'New Mexico', 'New Mexico', 'NM', 'US', 'state', 34.5199, -105.8701, true),
    ('US-NY', 'New York', 'New York', 'NY', 'US', 'state', 43.2994, -74.2179, true),
    ('US-NC', 'North Carolina', 'North Carolina', 'NC', 'US', 'state', 35.7596, -79.0193, true),
    ('US-ND', 'North Dakota', 'North Dakota', 'ND', 'US', 'state', 47.5515, -101.0020, true),
    ('US-OH', 'Ohio', 'Ohio', 'OH', 'US', 'state', 40.4173, -82.9071, true),
    ('US-OK', 'Oklahoma', 'Oklahoma', 'OK', 'US', 'state', 35.4676, -97.5164, true),
    ('US-OR', 'Oregon', 'Oregon', 'OR', 'US', 'state', 43.8041, -120.5542, true),
    ('US-PA', 'Pennsylvania', 'Pennsylvania', 'PA', 'US', 'state', 41.2033, -77.1945, true),
    ('US-RI', 'Rhode Island', 'Rhode Island', 'RI', 'US', 'state', 41.5801, -71.4774, true),
    ('US-SC', 'South Carolina', 'South Carolina', 'SC', 'US', 'state', 33.8361, -81.1637, true),
    ('US-SD', 'South Dakota', 'South Dakota', 'SD', 'US', 'state', 43.9695, -99.9018, true),
    ('US-TN', 'Tennessee', 'Tennessee', 'TN', 'US', 'state', 35.5175, -86.5804, true),
    ('US-TX', 'Texas', 'Texas', 'TX', 'US', 'state', 31.9686, -99.9018, true),
    ('US-UT', 'Utah', 'Utah', 'UT', 'US', 'state', 39.3210, -111.0937, true),
    ('US-VT', 'Vermont', 'Vermont', 'VT', 'US', 'state', 44.5588, -72.5778, true),
    ('US-VA', 'Virginia', 'Virginia', 'VA', 'US', 'state', 37.4316, -78.6569, true),
    ('US-WA', 'Washington', 'Washington', 'WA', 'US', 'state', 47.7511, -120.7401, true),
    ('US-WV', 'West Virginia', 'West Virginia', 'WV', 'US', 'state', 38.5976, -80.4549, true),
    ('US-WI', 'Wisconsin', 'Wisconsin', 'WI', 'US', 'state', 43.7844, -88.7879, true),
    ('US-WY', 'Wyoming', 'Wyoming', 'WY', 'US', 'state', 43.0750, -107.2903, true),
    ('US-DC', 'District of Columbia', 'District of Columbia', 'DC', 'US', 'district', 38.9072, -77.0369, true)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    native_name = EXCLUDED.native_name,
    code = EXCLUDED.code,
    type = EXCLUDED.type,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED STATES - INDIA STATES
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('IN-AN', 'Andaman and Nicobar Islands', 'अंडमान और निकोबार द्वीपसमूह', 'AN', 'IN', 'union territory', 11.7401, 92.6586, true),
    ('IN-AP', 'Andhra Pradesh', 'आंध्र प्रदेश', 'AP', 'IN', 'state', 15.9129, 79.7400, true),
    ('IN-AR', 'Arunachal Pradesh', 'अरुणाचल प्रदेश', 'AR', 'IN', 'state', 28.2180, 94.7278, true),
    ('IN-AS', 'Assam', 'असम', 'AS', 'IN', 'state', 26.2006, 92.9376, true),
    ('IN-BR', 'Bihar', 'बिहार', 'BR', 'IN', 'state', 25.0961, 85.3131, true),
    ('IN-CH', 'Chandigarh', 'चंडीगढ़', 'CH', 'IN', 'union territory', 30.7333, 76.7794, true),
    ('IN-CT', 'Chhattisgarh', 'छत्तीसगढ़', 'CT', 'IN', 'state', 21.2787, 81.8661, true),
    ('IN-DN', 'Dadra and Nagar Haveli and Daman and Diu', 'दादरा और नगर हवेली और दमन और दीव', 'DN', 'IN', 'union territory', 20.3974, 72.8328, true),
    ('IN-DL', 'Delhi', 'दिल्ली', 'DL', 'IN', 'union territory', 28.7041, 77.1025, true),
    ('IN-GA', 'Goa', 'गोवा', 'GA', 'IN', 'state', 15.2993, 74.1240, true),
    ('IN-GJ', 'Gujarat', 'गुजरात', 'GJ', 'IN', 'state', 22.2587, 71.1924, true),
    ('IN-HR', 'Haryana', 'हरियाणा', 'HR', 'IN', 'state', 29.0588, 76.0856, true),
    ('IN-HP', 'Himachal Pradesh', 'हिमाचल प्रदेश', 'HP', 'IN', 'state', 31.1048, 77.1734, true),
    ('IN-JK', 'Jammu and Kashmir', 'जम्मू और कश्मीर', 'JK', 'IN', 'union territory', 33.7782, 76.5762, true),
    ('IN-JH', 'Jharkhand', 'झारखंड', 'JH', 'IN', 'state', 23.6102, 85.2799, true),
    ('IN-KA', 'Karnataka', 'कर्नाटक', 'KA', 'IN', 'state', 15.3173, 75.7139, true),
    ('IN-KL', 'Kerala', 'केरल', 'KL', 'IN', 'state', 10.8505, 76.2711, true),
    ('IN-LA', 'Ladakh', 'लद्दाख', 'LA', 'IN', 'union territory', 34.1526, 77.5771, true),
    ('IN-LD', 'Lakshadweep', 'लक्षद्वीप', 'LD', 'IN', 'union territory', 10.5667, 72.6417, true),
    ('IN-MP', 'Madhya Pradesh', 'मध्य प्रदेश', 'MP', 'IN', 'state', 22.9734, 78.6569, true),
    ('IN-MH', 'Maharashtra', 'महाराष्ट्र', 'MH', 'IN', 'state', 19.7515, 75.7139, true),
    ('IN-MN', 'Manipur', 'मणिपुर', 'MN', 'IN', 'state', 24.6637, 93.9063, true),
    ('IN-ML', 'Meghalaya', 'मेघालय', 'ML', 'IN', 'state', 25.4670, 91.3662, true),
    ('IN-MZ', 'Mizoram', 'मिज़ोरम', 'MZ', 'IN', 'state', 23.1645, 92.9376, true),
    ('IN-NL', 'Nagaland', 'नागालैंड', 'NL', 'IN', 'state', 26.1584, 94.5624, true),
    ('IN-OR', 'Odisha', 'ओडिशा', 'OR', 'IN', 'state', 20.9517, 85.0985, true),
    ('IN-PY', 'Puducherry', 'पुडुचेरी', 'PY', 'IN', 'union territory', 11.9416, 79.8083, true),
    ('IN-PB', 'Punjab', 'पंजाब', 'PB', 'IN', 'state', 31.1471, 75.3412, true),
    ('IN-RJ', 'Rajasthan', 'राजस्थान', 'RJ', 'IN', 'state', 27.0238, 74.2179, true),
    ('IN-SK', 'Sikkim', 'सिक्किम', 'SK', 'IN', 'state', 27.5330, 88.5122, true),
    ('IN-TN', 'Tamil Nadu', 'तमिलनाडु', 'TN', 'IN', 'state', 11.1271, 78.6569, true),
    ('IN-TG', 'Telangana', 'तेलंगाना', 'TG', 'IN', 'state', 18.1124, 79.0193, true),
    ('IN-TR', 'Tripura', 'त्रिपुरा', 'TR', 'IN', 'state', 23.9408, 91.9882, true),
    ('IN-UP', 'Uttar Pradesh', 'उत्तर प्रदेश', 'UP', 'IN', 'state', 26.8467, 80.9462, true),
    ('IN-UT', 'Uttarakhand', 'उत्तराखंड', 'UT', 'IN', 'state', 30.0668, 79.0193, true),
    ('IN-WB', 'West Bengal', 'पश्चिम बंगाल', 'WB', 'IN', 'state', 22.9868, 87.8550, true)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    native_name = EXCLUDED.native_name,
    code = EXCLUDED.code,
    type = EXCLUDED.type,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED STATES - CANADA PROVINCES
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('CA-AB', 'Alberta', 'Alberta', 'AB', 'CA', 'province', 53.9333, -116.5765, true),
    ('CA-BC', 'British Columbia', 'British Columbia', 'BC', 'CA', 'province', 53.7267, -127.6476, true),
    ('CA-MB', 'Manitoba', 'Manitoba', 'MB', 'CA', 'province', 53.7609, -98.8139, true),
    ('CA-NB', 'New Brunswick', 'Nouveau-Brunswick', 'NB', 'CA', 'province', 46.5653, -66.4619, true),
    ('CA-NL', 'Newfoundland and Labrador', 'Terre-Neuve-et-Labrador', 'NL', 'CA', 'province', 53.1355, -57.6604, true),
    ('CA-NS', 'Nova Scotia', 'Nouvelle-Écosse', 'NS', 'CA', 'province', 44.6820, -63.7443, true),
    ('CA-ON', 'Ontario', 'Ontario', 'ON', 'CA', 'province', 51.2538, -85.3232, true),
    ('CA-PE', 'Prince Edward Island', 'Île-du-Prince-Édouard', 'PE', 'CA', 'province', 46.5107, -63.4168, true),
    ('CA-QC', 'Quebec', 'Québec', 'QC', 'CA', 'province', 52.9399, -73.5491, true),
    ('CA-SK', 'Saskatchewan', 'Saskatchewan', 'SK', 'CA', 'province', 52.9399, -106.4509, true),
    ('CA-NT', 'Northwest Territories', 'Territoires du Nord-Ouest', 'NT', 'CA', 'territory', 64.8255, -124.8457, true),
    ('CA-NU', 'Nunavut', 'Nunavut', 'NU', 'CA', 'territory', 70.2998, -83.1076, true),
    ('CA-YT', 'Yukon', 'Yukon', 'YT', 'CA', 'territory', 64.2823, -135.0000, true)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    native_name = EXCLUDED.native_name,
    code = EXCLUDED.code,
    type = EXCLUDED.type,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED STATES - AUSTRALIA STATES
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('AU-NSW', 'New South Wales', 'New South Wales', 'NSW', 'AU', 'state', -33.8688, 151.2093, true),
    ('AU-QLD', 'Queensland', 'Queensland', 'QLD', 'AU', 'state', -27.4698, 153.0251, true),
    ('AU-SA', 'South Australia', 'South Australia', 'SA', 'AU', 'state', -34.9285, 138.6007, true),
    ('AU-TAS', 'Tasmania', 'Tasmania', 'TAS', 'AU', 'state', -42.8821, 147.3272, true),
    ('AU-VIC', 'Victoria', 'Victoria', 'VIC', 'AU', 'state', -37.8136, 144.9631, true),
    ('AU-WA', 'Western Australia', 'Western Australia', 'WA', 'AU', 'state', -31.9505, 115.8605, true),
    ('AU-ACT', 'Australian Capital Territory', 'Australian Capital Territory', 'ACT', 'AU', 'territory', -35.2809, 149.1300, true),
    ('AU-NT', 'Northern Territory', 'Northern Territory', 'NT', 'AU', 'territory', -12.4634, 130.8456, true)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    native_name = EXCLUDED.native_name,
    code = EXCLUDED.code,
    type = EXCLUDED.type,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED STATES - UK REGIONS
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('GB-ENG', 'England', 'England', 'ENG', 'GB', 'country', 52.3555, -1.1743, true),
    ('GB-SCT', 'Scotland', 'Scotland', 'SCT', 'GB', 'country', 56.4907, -4.2026, true),
    ('GB-WLS', 'Wales', 'Cymru', 'WLS', 'GB', 'country', 52.1307, -3.7837, true),
    ('GB-NIR', 'Northern Ireland', 'Northern Ireland', 'NIR', 'GB', 'province', 54.7877, -6.4923, true)
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    native_name = EXCLUDED.native_name,
    code = EXCLUDED.code,
    type = EXCLUDED.type,
    latitude = EXCLUDED.latitude,
    longitude = EXCLUDED.longitude,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- Record this migration
INSERT INTO schema_migrations (version, dirty) VALUES (2, false) ON CONFLICT (version) DO NOTHING;
