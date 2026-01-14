-- Migration: 000003_expand_world_data
-- Description: Expand location data to cover more countries and states worldwide
-- Created: 2024-01-01

SET search_path TO location, public;

-- =====================================================
-- EXPAND COUNTRIES - Additional Countries
-- =====================================================
INSERT INTO countries (id, name, native_name, capital, region, subregion, currency, languages, calling_code, flag_emoji, latitude, longitude, active)
VALUES
    -- Europe
    ('AT', 'Austria', 'Österreich', 'Vienna', 'Europe', 'Western Europe', 'EUR', '["de"]', '+43', '🇦🇹', 47.5162, 14.5501, true),
    ('BE', 'Belgium', 'België', 'Brussels', 'Europe', 'Western Europe', 'EUR', '["nl","fr","de"]', '+32', '🇧🇪', 50.5039, 4.4699, true),
    ('CZ', 'Czech Republic', 'Česká republika', 'Prague', 'Europe', 'Central Europe', 'CZK', '["cs"]', '+420', '🇨🇿', 49.8175, 15.4729, true),
    ('DK', 'Denmark', 'Danmark', 'Copenhagen', 'Europe', 'Northern Europe', 'DKK', '["da"]', '+45', '🇩🇰', 56.2639, 9.5018, true),
    ('FI', 'Finland', 'Suomi', 'Helsinki', 'Europe', 'Northern Europe', 'EUR', '["fi","sv"]', '+358', '🇫🇮', 61.9241, 25.7482, true),
    ('GR', 'Greece', 'Ελλάδα', 'Athens', 'Europe', 'Southern Europe', 'EUR', '["el"]', '+30', '🇬🇷', 39.0742, 21.8243, true),
    ('HU', 'Hungary', 'Magyarország', 'Budapest', 'Europe', 'Central Europe', 'HUF', '["hu"]', '+36', '🇭🇺', 47.1625, 19.5033, true),
    ('NO', 'Norway', 'Norge', 'Oslo', 'Europe', 'Northern Europe', 'NOK', '["no"]', '+47', '🇳🇴', 60.4720, 8.4689, true),
    ('PL', 'Poland', 'Polska', 'Warsaw', 'Europe', 'Central Europe', 'PLN', '["pl"]', '+48', '🇵🇱', 51.9194, 19.1451, true),
    ('PT', 'Portugal', 'Portugal', 'Lisbon', 'Europe', 'Southern Europe', 'EUR', '["pt"]', '+351', '🇵🇹', 39.3999, -8.2245, true),
    ('RO', 'Romania', 'România', 'Bucharest', 'Europe', 'Eastern Europe', 'RON', '["ro"]', '+40', '🇷🇴', 45.9432, 24.9668, true),
    ('RU', 'Russia', 'Россия', 'Moscow', 'Europe', 'Eastern Europe', 'RUB', '["ru"]', '+7', '🇷🇺', 61.5240, 105.3188, true),
    ('UA', 'Ukraine', 'Україна', 'Kyiv', 'Europe', 'Eastern Europe', 'UAH', '["uk"]', '+380', '🇺🇦', 48.3794, 31.1656, true),
    -- Middle East
    ('IL', 'Israel', 'ישראל', 'Jerusalem', 'Asia', 'Western Asia', 'ILS', '["he","ar"]', '+972', '🇮🇱', 31.0461, 34.8516, true),
    ('SA', 'Saudi Arabia', 'السعودية', 'Riyadh', 'Asia', 'Western Asia', 'SAR', '["ar"]', '+966', '🇸🇦', 23.8859, 45.0792, true),
    ('TR', 'Turkey', 'Türkiye', 'Ankara', 'Asia', 'Western Asia', 'TRY', '["tr"]', '+90', '🇹🇷', 38.9637, 35.2433, true),
    ('QA', 'Qatar', 'قطر', 'Doha', 'Asia', 'Western Asia', 'QAR', '["ar"]', '+974', '🇶🇦', 25.3548, 51.1839, true),
    ('KW', 'Kuwait', 'الكويت', 'Kuwait City', 'Asia', 'Western Asia', 'KWD', '["ar"]', '+965', '🇰🇼', 29.3117, 47.4818, true),
    ('BH', 'Bahrain', 'البحرين', 'Manama', 'Asia', 'Western Asia', 'BHD', '["ar"]', '+973', '🇧🇭', 26.0667, 50.5577, true),
    ('OM', 'Oman', 'عُمان', 'Muscat', 'Asia', 'Western Asia', 'OMR', '["ar"]', '+968', '🇴🇲', 21.4735, 55.9754, true),
    -- Asia
    ('TH', 'Thailand', 'ประเทศไทย', 'Bangkok', 'Asia', 'South-Eastern Asia', 'THB', '["th"]', '+66', '🇹🇭', 15.8700, 100.9925, true),
    ('VN', 'Vietnam', 'Việt Nam', 'Hanoi', 'Asia', 'South-Eastern Asia', 'VND', '["vi"]', '+84', '🇻🇳', 14.0583, 108.2772, true),
    ('BD', 'Bangladesh', 'বাংলাদেশ', 'Dhaka', 'Asia', 'Southern Asia', 'BDT', '["bn"]', '+880', '🇧🇩', 23.6850, 90.3563, true),
    ('PK', 'Pakistan', 'پاکستان', 'Islamabad', 'Asia', 'Southern Asia', 'PKR', '["ur","en"]', '+92', '🇵🇰', 30.3753, 69.3451, true),
    ('LK', 'Sri Lanka', 'ශ්‍රී ලංකාව', 'Colombo', 'Asia', 'Southern Asia', 'LKR', '["si","ta"]', '+94', '🇱🇰', 7.8731, 80.7718, true),
    ('NP', 'Nepal', 'नेपाल', 'Kathmandu', 'Asia', 'Southern Asia', 'NPR', '["ne"]', '+977', '🇳🇵', 28.3949, 84.1240, true),
    ('MM', 'Myanmar', 'မြန်မာ', 'Naypyidaw', 'Asia', 'South-Eastern Asia', 'MMK', '["my"]', '+95', '🇲🇲', 21.9162, 95.9560, true),
    ('KH', 'Cambodia', 'កម្ពុជា', 'Phnom Penh', 'Asia', 'South-Eastern Asia', 'KHR', '["km"]', '+855', '🇰🇭', 12.5657, 104.9910, true),
    ('HK', 'Hong Kong', '香港', 'Hong Kong', 'Asia', 'Eastern Asia', 'HKD', '["zh","en"]', '+852', '🇭🇰', 22.3193, 114.1694, true),
    ('TW', 'Taiwan', '臺灣', 'Taipei', 'Asia', 'Eastern Asia', 'TWD', '["zh"]', '+886', '🇹🇼', 23.6978, 120.9605, true),
    -- Africa
    ('EG', 'Egypt', 'مصر', 'Cairo', 'Africa', 'Northern Africa', 'EGP', '["ar"]', '+20', '🇪🇬', 26.8206, 30.8025, true),
    ('NG', 'Nigeria', 'Nigeria', 'Abuja', 'Africa', 'Western Africa', 'NGN', '["en"]', '+234', '🇳🇬', 9.0820, 8.6753, true),
    ('KE', 'Kenya', 'Kenya', 'Nairobi', 'Africa', 'Eastern Africa', 'KES', '["en","sw"]', '+254', '🇰🇪', -0.0236, 37.9062, true),
    ('GH', 'Ghana', 'Ghana', 'Accra', 'Africa', 'Western Africa', 'GHS', '["en"]', '+233', '🇬🇭', 7.9465, -1.0232, true),
    ('MA', 'Morocco', 'المغرب', 'Rabat', 'Africa', 'Northern Africa', 'MAD', '["ar","fr"]', '+212', '🇲🇦', 31.7917, -7.0926, true),
    ('TN', 'Tunisia', 'تونس', 'Tunis', 'Africa', 'Northern Africa', 'TND', '["ar"]', '+216', '🇹🇳', 33.8869, 9.5375, true),
    ('TZ', 'Tanzania', 'Tanzania', 'Dodoma', 'Africa', 'Eastern Africa', 'TZS', '["sw","en"]', '+255', '🇹🇿', -6.3690, 34.8888, true),
    ('UG', 'Uganda', 'Uganda', 'Kampala', 'Africa', 'Eastern Africa', 'UGX', '["en","sw"]', '+256', '🇺🇬', 1.3733, 32.2903, true),
    ('ET', 'Ethiopia', 'ኢትዮጵያ', 'Addis Ababa', 'Africa', 'Eastern Africa', 'ETB', '["am"]', '+251', '🇪🇹', 9.1450, 40.4897, true),
    -- Americas
    ('AR', 'Argentina', 'Argentina', 'Buenos Aires', 'Americas', 'South America', 'ARS', '["es"]', '+54', '🇦🇷', -38.4161, -63.6167, true),
    ('CL', 'Chile', 'Chile', 'Santiago', 'Americas', 'South America', 'CLP', '["es"]', '+56', '🇨🇱', -35.6751, -71.5430, true),
    ('CO', 'Colombia', 'Colombia', 'Bogotá', 'Americas', 'South America', 'COP', '["es"]', '+57', '🇨🇴', 4.5709, -74.2973, true),
    ('PE', 'Peru', 'Perú', 'Lima', 'Americas', 'South America', 'PEN', '["es"]', '+51', '🇵🇪', -9.1900, -75.0152, true),
    ('VE', 'Venezuela', 'Venezuela', 'Caracas', 'Americas', 'South America', 'VES', '["es"]', '+58', '🇻🇪', 6.4238, -66.5897, true),
    ('EC', 'Ecuador', 'Ecuador', 'Quito', 'Americas', 'South America', 'USD', '["es"]', '+593', '🇪🇨', -1.8312, -78.1834, true),
    ('UY', 'Uruguay', 'Uruguay', 'Montevideo', 'Americas', 'South America', 'UYU', '["es"]', '+598', '🇺🇾', -32.5228, -55.7658, true),
    ('PA', 'Panama', 'Panamá', 'Panama City', 'Americas', 'Central America', 'PAB', '["es"]', '+507', '🇵🇦', 8.5380, -80.7821, true),
    ('CR', 'Costa Rica', 'Costa Rica', 'San José', 'Americas', 'Central America', 'CRC', '["es"]', '+506', '🇨🇷', 9.7489, -83.7534, true),
    ('PR', 'Puerto Rico', 'Puerto Rico', 'San Juan', 'Americas', 'Caribbean', 'USD', '["es","en"]', '+1', '🇵🇷', 18.2208, -66.5901, true),
    ('JM', 'Jamaica', 'Jamaica', 'Kingston', 'Americas', 'Caribbean', 'JMD', '["en"]', '+1', '🇯🇲', 18.1096, -77.2975, true),
    ('DO', 'Dominican Republic', 'República Dominicana', 'Santo Domingo', 'Americas', 'Caribbean', 'DOP', '["es"]', '+1', '🇩🇴', 18.7357, -70.1627, true),
    -- Oceania
    ('FJ', 'Fiji', 'Fiji', 'Suva', 'Oceania', 'Melanesia', 'FJD', '["en","fj","hi"]', '+679', '🇫🇯', -17.7134, 178.0650, true)
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
-- EXPAND CURRENCIES
-- =====================================================
INSERT INTO currencies (code, name, symbol, decimal_places, active)
VALUES
    ('CZK', 'Czech Koruna', 'Kč', 2, true),
    ('DKK', 'Danish Krone', 'kr', 2, true),
    ('HUF', 'Hungarian Forint', 'Ft', 0, true),
    ('NOK', 'Norwegian Krone', 'kr', 2, true),
    ('PLN', 'Polish Złoty', 'zł', 2, true),
    ('RON', 'Romanian Leu', 'lei', 2, true),
    ('RUB', 'Russian Ruble', '₽', 2, true),
    ('UAH', 'Ukrainian Hryvnia', '₴', 2, true),
    ('ILS', 'Israeli Shekel', '₪', 2, true),
    ('SAR', 'Saudi Riyal', '﷼', 2, true),
    ('TRY', 'Turkish Lira', '₺', 2, true),
    ('QAR', 'Qatari Riyal', '﷼', 2, true),
    ('KWD', 'Kuwaiti Dinar', 'د.ك', 3, true),
    ('BHD', 'Bahraini Dinar', '.د.ب', 3, true),
    ('OMR', 'Omani Rial', '﷼', 3, true),
    ('THB', 'Thai Baht', '฿', 2, true),
    ('VND', 'Vietnamese Dong', '₫', 0, true),
    ('BDT', 'Bangladeshi Taka', '৳', 2, true),
    ('PKR', 'Pakistani Rupee', '₨', 2, true),
    ('LKR', 'Sri Lankan Rupee', '₨', 2, true),
    ('NPR', 'Nepalese Rupee', '₨', 2, true),
    ('HKD', 'Hong Kong Dollar', 'HK$', 2, true),
    ('TWD', 'New Taiwan Dollar', 'NT$', 2, true),
    ('EGP', 'Egyptian Pound', 'E£', 2, true),
    ('NGN', 'Nigerian Naira', '₦', 2, true),
    ('KES', 'Kenyan Shilling', 'KSh', 2, true),
    ('GHS', 'Ghanaian Cedi', 'GH₵', 2, true),
    ('MAD', 'Moroccan Dirham', 'د.م.', 2, true),
    ('ARS', 'Argentine Peso', '$', 2, true),
    ('CLP', 'Chilean Peso', '$', 0, true),
    ('COP', 'Colombian Peso', '$', 2, true),
    ('PEN', 'Peruvian Sol', 'S/', 2, true),
    ('CRC', 'Costa Rican Colón', '₡', 2, true),
    ('FJD', 'Fijian Dollar', 'FJ$', 2, true)
ON CONFLICT (code) DO UPDATE SET
    name = EXCLUDED.name,
    symbol = EXCLUDED.symbol,
    decimal_places = EXCLUDED.decimal_places,
    active = EXCLUDED.active,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- EXPAND TIMEZONES
-- =====================================================
INSERT INTO timezones (id, name, abbreviation, "offset", dst, countries)
VALUES
    -- Europe additional
    ('Europe/Vienna', 'Central European Time', 'CET/CEST', '+01:00', true, '["AT"]'),
    ('Europe/Brussels', 'Central European Time', 'CET/CEST', '+01:00', true, '["BE"]'),
    ('Europe/Prague', 'Central European Time', 'CET/CEST', '+01:00', true, '["CZ"]'),
    ('Europe/Copenhagen', 'Central European Time', 'CET/CEST', '+01:00', true, '["DK"]'),
    ('Europe/Helsinki', 'Eastern European Time', 'EET/EEST', '+02:00', true, '["FI"]'),
    ('Europe/Athens', 'Eastern European Time', 'EET/EEST', '+02:00', true, '["GR"]'),
    ('Europe/Budapest', 'Central European Time', 'CET/CEST', '+01:00', true, '["HU"]'),
    ('Europe/Oslo', 'Central European Time', 'CET/CEST', '+01:00', true, '["NO"]'),
    ('Europe/Warsaw', 'Central European Time', 'CET/CEST', '+01:00', true, '["PL"]'),
    ('Europe/Lisbon', 'Western European Time', 'WET/WEST', '+00:00', true, '["PT"]'),
    ('Europe/Bucharest', 'Eastern European Time', 'EET/EEST', '+02:00', true, '["RO"]'),
    ('Europe/Moscow', 'Moscow Standard Time', 'MSK', '+03:00', false, '["RU"]'),
    ('Europe/Kyiv', 'Eastern European Time', 'EET/EEST', '+02:00', true, '["UA"]'),
    -- Middle East
    ('Asia/Jerusalem', 'Israel Standard Time', 'IST/IDT', '+02:00', true, '["IL"]'),
    ('Asia/Riyadh', 'Arabian Standard Time', 'AST', '+03:00', false, '["SA"]'),
    ('Europe/Istanbul', 'Turkey Time', 'TRT', '+03:00', false, '["TR"]'),
    ('Asia/Qatar', 'Arabian Standard Time', 'AST', '+03:00', false, '["QA"]'),
    ('Asia/Kuwait', 'Arabian Standard Time', 'AST', '+03:00', false, '["KW"]'),
    ('Asia/Bahrain', 'Arabian Standard Time', 'AST', '+03:00', false, '["BH"]'),
    ('Asia/Muscat', 'Gulf Standard Time', 'GST', '+04:00', false, '["OM"]'),
    -- Asia additional
    ('Asia/Bangkok', 'Indochina Time', 'ICT', '+07:00', false, '["TH"]'),
    ('Asia/Ho_Chi_Minh', 'Indochina Time', 'ICT', '+07:00', false, '["VN"]'),
    ('Asia/Dhaka', 'Bangladesh Standard Time', 'BST', '+06:00', false, '["BD"]'),
    ('Asia/Karachi', 'Pakistan Standard Time', 'PKT', '+05:00', false, '["PK"]'),
    ('Asia/Colombo', 'India Standard Time', 'IST', '+05:30', false, '["LK"]'),
    ('Asia/Kathmandu', 'Nepal Time', 'NPT', '+05:45', false, '["NP"]'),
    ('Asia/Taipei', 'Taipei Standard Time', 'CST', '+08:00', false, '["TW"]'),
    -- Africa
    ('Africa/Cairo', 'Eastern European Time', 'EET', '+02:00', false, '["EG"]'),
    ('Africa/Lagos', 'West Africa Time', 'WAT', '+01:00', false, '["NG","GH"]'),
    ('Africa/Nairobi', 'East Africa Time', 'EAT', '+03:00', false, '["KE","TZ","UG","ET"]'),
    ('Africa/Casablanca', 'Western European Time', 'WET/WEST', '+00:00', true, '["MA"]'),
    -- Americas additional
    ('America/Buenos_Aires', 'Argentina Time', 'ART', '-03:00', false, '["AR"]'),
    ('America/Santiago', 'Chile Standard Time', 'CLT/CLST', '-04:00', true, '["CL"]'),
    ('America/Bogota', 'Colombia Time', 'COT', '-05:00', false, '["CO"]'),
    ('America/Lima', 'Peru Time', 'PET', '-05:00', false, '["PE"]'),
    ('America/Caracas', 'Venezuela Time', 'VET', '-04:00', false, '["VE"]'),
    ('America/Panama', 'Eastern Standard Time', 'EST', '-05:00', false, '["PA"]'),
    ('America/Costa_Rica', 'Central Standard Time', 'CST', '-06:00', false, '["CR"]'),
    ('America/Puerto_Rico', 'Atlantic Standard Time', 'AST', '-04:00', false, '["PR"]'),
    ('America/Jamaica', 'Eastern Standard Time', 'EST', '-05:00', false, '["JM"]'),
    ('America/Santo_Domingo', 'Atlantic Standard Time', 'AST', '-04:00', false, '["DO"]'),
    -- Oceania
    ('Pacific/Fiji', 'Fiji Time', 'FJT', '+12:00', true, '["FJ"]')
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    abbreviation = EXCLUDED.abbreviation,
    "offset" = EXCLUDED."offset",
    dst = EXCLUDED.dst,
    countries = EXCLUDED.countries,
    updated_at = CURRENT_TIMESTAMP;

-- =====================================================
-- SEED STATES - GERMANY (Bundesländer)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('DE-BW', 'Baden-Württemberg', 'Baden-Württemberg', 'BW', 'DE', 'state', 48.6616, 9.3501, true),
    ('DE-BY', 'Bavaria', 'Bayern', 'BY', 'DE', 'state', 48.7904, 11.4979, true),
    ('DE-BE', 'Berlin', 'Berlin', 'BE', 'DE', 'state', 52.5200, 13.4050, true),
    ('DE-BB', 'Brandenburg', 'Brandenburg', 'BB', 'DE', 'state', 52.4125, 12.5316, true),
    ('DE-HB', 'Bremen', 'Bremen', 'HB', 'DE', 'state', 53.0793, 8.8017, true),
    ('DE-HH', 'Hamburg', 'Hamburg', 'HH', 'DE', 'state', 53.5511, 9.9937, true),
    ('DE-HE', 'Hesse', 'Hessen', 'HE', 'DE', 'state', 50.6521, 9.1624, true),
    ('DE-MV', 'Mecklenburg-Vorpommern', 'Mecklenburg-Vorpommern', 'MV', 'DE', 'state', 53.6127, 12.4296, true),
    ('DE-NI', 'Lower Saxony', 'Niedersachsen', 'NI', 'DE', 'state', 52.6367, 9.8451, true),
    ('DE-NW', 'North Rhine-Westphalia', 'Nordrhein-Westfalen', 'NW', 'DE', 'state', 51.4332, 7.6616, true),
    ('DE-RP', 'Rhineland-Palatinate', 'Rheinland-Pfalz', 'RP', 'DE', 'state', 50.1183, 7.3090, true),
    ('DE-SL', 'Saarland', 'Saarland', 'SL', 'DE', 'state', 49.3964, 7.0230, true),
    ('DE-SN', 'Saxony', 'Sachsen', 'SN', 'DE', 'state', 51.1045, 13.2017, true),
    ('DE-ST', 'Saxony-Anhalt', 'Sachsen-Anhalt', 'ST', 'DE', 'state', 51.9503, 11.6923, true),
    ('DE-SH', 'Schleswig-Holstein', 'Schleswig-Holstein', 'SH', 'DE', 'state', 54.2194, 9.6961, true),
    ('DE-TH', 'Thuringia', 'Thüringen', 'TH', 'DE', 'state', 50.9014, 11.0348, true)
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
-- SEED STATES - FRANCE (Régions)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('FR-ARA', 'Auvergne-Rhône-Alpes', 'Auvergne-Rhône-Alpes', 'ARA', 'FR', 'region', 45.4473, 4.3859, true),
    ('FR-BFC', 'Bourgogne-Franche-Comté', 'Bourgogne-Franche-Comté', 'BFC', 'FR', 'region', 47.2805, 4.9994, true),
    ('FR-BRE', 'Brittany', 'Bretagne', 'BRE', 'FR', 'region', 48.2020, -2.9326, true),
    ('FR-CVL', 'Centre-Val de Loire', 'Centre-Val de Loire', 'CVL', 'FR', 'region', 47.7516, 1.6751, true),
    ('FR-COR', 'Corsica', 'Corse', 'COR', 'FR', 'region', 42.0396, 9.0129, true),
    ('FR-GES', 'Grand Est', 'Grand Est', 'GES', 'FR', 'region', 48.6998, 6.1878, true),
    ('FR-HDF', 'Hauts-de-France', 'Hauts-de-France', 'HDF', 'FR', 'region', 49.9659, 2.7644, true),
    ('FR-IDF', 'Île-de-France', 'Île-de-France', 'IDF', 'FR', 'region', 48.8499, 2.6370, true),
    ('FR-NOR', 'Normandy', 'Normandie', 'NOR', 'FR', 'region', 49.1829, -0.3707, true),
    ('FR-NAQ', 'Nouvelle-Aquitaine', 'Nouvelle-Aquitaine', 'NAQ', 'FR', 'region', 45.7087, 0.6266, true),
    ('FR-OCC', 'Occitanie', 'Occitanie', 'OCC', 'FR', 'region', 43.8927, 3.2828, true),
    ('FR-PDL', 'Pays de la Loire', 'Pays de la Loire', 'PDL', 'FR', 'region', 47.7632, -0.3299, true),
    ('FR-PAC', 'Provence-Alpes-Côte d''Azur', 'Provence-Alpes-Côte d''Azur', 'PAC', 'FR', 'region', 43.9352, 6.0679, true)
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
-- SEED STATES - JAPAN (Prefectures - Major ones)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('JP-13', 'Tokyo', '東京都', '13', 'JP', 'prefecture', 35.6762, 139.6503, true),
    ('JP-27', 'Osaka', '大阪府', '27', 'JP', 'prefecture', 34.6937, 135.5023, true),
    ('JP-14', 'Kanagawa', '神奈川県', '14', 'JP', 'prefecture', 35.4478, 139.6425, true),
    ('JP-23', 'Aichi', '愛知県', '23', 'JP', 'prefecture', 35.1802, 136.9066, true),
    ('JP-11', 'Saitama', '埼玉県', '11', 'JP', 'prefecture', 35.8570, 139.6489, true),
    ('JP-12', 'Chiba', '千葉県', '12', 'JP', 'prefecture', 35.6050, 140.1233, true),
    ('JP-28', 'Hyogo', '兵庫県', '28', 'JP', 'prefecture', 34.6913, 135.1830, true),
    ('JP-01', 'Hokkaido', '北海道', '01', 'JP', 'prefecture', 43.0642, 141.3469, true),
    ('JP-40', 'Fukuoka', '福岡県', '40', 'JP', 'prefecture', 33.5904, 130.4017, true),
    ('JP-26', 'Kyoto', '京都府', '26', 'JP', 'prefecture', 35.0116, 135.7681, true)
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
-- SEED STATES - BRAZIL (States - Major ones)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('BR-SP', 'São Paulo', 'São Paulo', 'SP', 'BR', 'state', -23.5505, -46.6333, true),
    ('BR-RJ', 'Rio de Janeiro', 'Rio de Janeiro', 'RJ', 'BR', 'state', -22.9068, -43.1729, true),
    ('BR-MG', 'Minas Gerais', 'Minas Gerais', 'MG', 'BR', 'state', -18.5122, -44.5550, true),
    ('BR-BA', 'Bahia', 'Bahia', 'BA', 'BR', 'state', -12.5797, -41.7007, true),
    ('BR-RS', 'Rio Grande do Sul', 'Rio Grande do Sul', 'RS', 'BR', 'state', -30.0346, -51.2177, true),
    ('BR-PR', 'Paraná', 'Paraná', 'PR', 'BR', 'state', -25.2521, -52.0215, true),
    ('BR-PE', 'Pernambuco', 'Pernambuco', 'PE', 'BR', 'state', -8.0476, -34.8770, true),
    ('BR-CE', 'Ceará', 'Ceará', 'CE', 'BR', 'state', -3.7172, -38.5433, true),
    ('BR-DF', 'Federal District', 'Distrito Federal', 'DF', 'BR', 'federal district', -15.8267, -47.9218, true),
    ('BR-SC', 'Santa Catarina', 'Santa Catarina', 'SC', 'BR', 'state', -27.2423, -50.2189, true)
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
-- SEED STATES - MEXICO (States - Major ones)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('MX-CMX', 'Mexico City', 'Ciudad de México', 'CMX', 'MX', 'federal district', 19.4326, -99.1332, true),
    ('MX-JAL', 'Jalisco', 'Jalisco', 'JAL', 'MX', 'state', 20.6595, -103.3494, true),
    ('MX-NLE', 'Nuevo León', 'Nuevo León', 'NLE', 'MX', 'state', 25.5922, -99.9962, true),
    ('MX-MEX', 'State of Mexico', 'Estado de México', 'MEX', 'MX', 'state', 19.4969, -99.7233, true),
    ('MX-VER', 'Veracruz', 'Veracruz', 'VER', 'MX', 'state', 19.1738, -96.1342, true),
    ('MX-PUE', 'Puebla', 'Puebla', 'PUE', 'MX', 'state', 19.0414, -98.2063, true),
    ('MX-GUA', 'Guanajuato', 'Guanajuato', 'GUA', 'MX', 'state', 21.0190, -101.2574, true),
    ('MX-CHH', 'Chihuahua', 'Chihuahua', 'CHH', 'MX', 'state', 28.6330, -106.0691, true),
    ('MX-BCN', 'Baja California', 'Baja California', 'BCN', 'MX', 'state', 30.8406, -115.2838, true),
    ('MX-QRO', 'Querétaro', 'Querétaro', 'QRO', 'MX', 'state', 20.5888, -100.3899, true)
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
-- SEED STATES - UAE (Emirates)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('AE-AZ', 'Abu Dhabi', 'أبوظبي', 'AZ', 'AE', 'emirate', 24.4539, 54.3773, true),
    ('AE-DU', 'Dubai', 'دبي', 'DU', 'AE', 'emirate', 25.2048, 55.2708, true),
    ('AE-SH', 'Sharjah', 'الشارقة', 'SH', 'AE', 'emirate', 25.3575, 55.4033, true),
    ('AE-AJ', 'Ajman', 'عجمان', 'AJ', 'AE', 'emirate', 25.4052, 55.5136, true),
    ('AE-UQ', 'Umm Al Quwain', 'أم القيوين', 'UQ', 'AE', 'emirate', 25.5647, 55.5553, true),
    ('AE-RK', 'Ras Al Khaimah', 'رأس الخيمة', 'RK', 'AE', 'emirate', 25.7895, 55.9432, true),
    ('AE-FU', 'Fujairah', 'الفجيرة', 'FU', 'AE', 'emirate', 25.1288, 56.3265, true)
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
-- SEED STATES - SPAIN (Autonomous Communities)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('ES-AN', 'Andalusia', 'Andalucía', 'AN', 'ES', 'autonomous community', 37.5443, -4.7278, true),
    ('ES-AR', 'Aragon', 'Aragón', 'AR', 'ES', 'autonomous community', 41.5976, -0.9057, true),
    ('ES-AS', 'Asturias', 'Asturias', 'AS', 'ES', 'autonomous community', 43.3614, -5.8593, true),
    ('ES-CB', 'Cantabria', 'Cantabria', 'CB', 'ES', 'autonomous community', 43.1828, -3.9878, true),
    ('ES-CL', 'Castile and León', 'Castilla y León', 'CL', 'ES', 'autonomous community', 41.8357, -4.3976, true),
    ('ES-CM', 'Castilla-La Mancha', 'Castilla-La Mancha', 'CM', 'ES', 'autonomous community', 39.2796, -3.0977, true),
    ('ES-CT', 'Catalonia', 'Catalunya', 'CT', 'ES', 'autonomous community', 41.5912, 1.5209, true),
    ('ES-EX', 'Extremadura', 'Extremadura', 'EX', 'ES', 'autonomous community', 39.4937, -6.0679, true),
    ('ES-GA', 'Galicia', 'Galicia', 'GA', 'ES', 'autonomous community', 42.5751, -8.1339, true),
    ('ES-IB', 'Balearic Islands', 'Illes Balears', 'IB', 'ES', 'autonomous community', 39.5696, 2.6502, true),
    ('ES-CN', 'Canary Islands', 'Canarias', 'CN', 'ES', 'autonomous community', 28.2916, -16.6291, true),
    ('ES-RI', 'La Rioja', 'La Rioja', 'RI', 'ES', 'autonomous community', 42.2871, -2.5396, true),
    ('ES-MD', 'Community of Madrid', 'Comunidad de Madrid', 'MD', 'ES', 'autonomous community', 40.4168, -3.7038, true),
    ('ES-MC', 'Region of Murcia', 'Región de Murcia', 'MC', 'ES', 'autonomous community', 37.9922, -1.1307, true),
    ('ES-NC', 'Navarre', 'Navarra', 'NC', 'ES', 'autonomous community', 42.6954, -1.6761, true),
    ('ES-PV', 'Basque Country', 'País Vasco', 'PV', 'ES', 'autonomous community', 42.9896, -2.6189, true),
    ('ES-VC', 'Valencian Community', 'Comunitat Valenciana', 'VC', 'ES', 'autonomous community', 39.4840, -0.7533, true)
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
-- SEED STATES - ITALY (Regions)
-- =====================================================
INSERT INTO states (id, name, native_name, code, country_id, type, latitude, longitude, active)
VALUES
    ('IT-65', 'Abruzzo', 'Abruzzo', '65', 'IT', 'region', 42.1920, 13.7289, true),
    ('IT-77', 'Basilicata', 'Basilicata', '77', 'IT', 'region', 40.6431, 15.9694, true),
    ('IT-78', 'Calabria', 'Calabria', '78', 'IT', 'region', 38.9058, 16.5946, true),
    ('IT-72', 'Campania', 'Campania', '72', 'IT', 'region', 40.8518, 14.2681, true),
    ('IT-45', 'Emilia-Romagna', 'Emilia-Romagna', '45', 'IT', 'region', 44.4949, 11.3426, true),
    ('IT-36', 'Friuli Venezia Giulia', 'Friuli Venezia Giulia', '36', 'IT', 'region', 46.0651, 13.2354, true),
    ('IT-62', 'Lazio', 'Lazio', '62', 'IT', 'region', 41.9028, 12.4964, true),
    ('IT-42', 'Liguria', 'Liguria', '42', 'IT', 'region', 44.3168, 8.3965, true),
    ('IT-25', 'Lombardy', 'Lombardia', '25', 'IT', 'region', 45.4654, 9.1859, true),
    ('IT-57', 'Marche', 'Marche', '57', 'IT', 'region', 43.6158, 13.5189, true),
    ('IT-67', 'Molise', 'Molise', '67', 'IT', 'region', 41.5608, 14.6684, true),
    ('IT-21', 'Piedmont', 'Piemonte', '21', 'IT', 'region', 45.0703, 7.6869, true),
    ('IT-75', 'Apulia', 'Puglia', '75', 'IT', 'region', 41.1257, 16.8667, true),
    ('IT-88', 'Sardinia', 'Sardegna', '88', 'IT', 'region', 40.1209, 9.0129, true),
    ('IT-82', 'Sicily', 'Sicilia', '82', 'IT', 'region', 37.5994, 14.0154, true),
    ('IT-52', 'Tuscany', 'Toscana', '52', 'IT', 'region', 43.7711, 11.2486, true),
    ('IT-32', 'Trentino-Alto Adige', 'Trentino-Alto Adige', '32', 'IT', 'region', 46.4337, 11.1693, true),
    ('IT-55', 'Umbria', 'Umbria', '55', 'IT', 'region', 42.9384, 12.6216, true),
    ('IT-23', 'Aosta Valley', 'Valle d''Aosta', '23', 'IT', 'region', 45.7389, 7.4262, true),
    ('IT-34', 'Veneto', 'Veneto', '34', 'IT', 'region', 45.4415, 12.3155, true)
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
INSERT INTO schema_migrations (version, dirty) VALUES (3, false) ON CONFLICT (version) DO NOTHING;
