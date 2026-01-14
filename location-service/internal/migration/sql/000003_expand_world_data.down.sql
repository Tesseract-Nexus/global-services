-- Migration: 000003_expand_world_data (DOWN)
-- Description: Rollback expanded location data
-- Created: 2024-01-01

SET search_path TO location, public;

-- Delete expanded states
DELETE FROM states WHERE id LIKE 'DE-%';
DELETE FROM states WHERE id LIKE 'FR-%';
DELETE FROM states WHERE id LIKE 'JP-%';
DELETE FROM states WHERE id LIKE 'BR-%';
DELETE FROM states WHERE id LIKE 'MX-%';
DELETE FROM states WHERE id LIKE 'AE-%';
DELETE FROM states WHERE id LIKE 'ES-%';
DELETE FROM states WHERE id LIKE 'IT-%';

-- Delete expanded timezones
DELETE FROM timezones WHERE id IN (
    'Europe/Vienna', 'Europe/Brussels', 'Europe/Prague', 'Europe/Copenhagen',
    'Europe/Helsinki', 'Europe/Athens', 'Europe/Budapest', 'Europe/Oslo',
    'Europe/Warsaw', 'Europe/Lisbon', 'Europe/Bucharest', 'Europe/Moscow',
    'Europe/Kyiv', 'Asia/Jerusalem', 'Asia/Riyadh', 'Europe/Istanbul',
    'Asia/Qatar', 'Asia/Kuwait', 'Asia/Bahrain', 'Asia/Muscat',
    'Asia/Bangkok', 'Asia/Ho_Chi_Minh', 'Asia/Dhaka', 'Asia/Karachi',
    'Asia/Colombo', 'Asia/Kathmandu', 'Asia/Taipei', 'Africa/Cairo',
    'Africa/Lagos', 'Africa/Nairobi', 'Africa/Casablanca',
    'America/Buenos_Aires', 'America/Santiago', 'America/Bogota',
    'America/Lima', 'America/Caracas', 'America/Panama', 'America/Costa_Rica',
    'America/Puerto_Rico', 'America/Jamaica', 'America/Santo_Domingo',
    'Pacific/Fiji'
);

-- Delete expanded currencies
DELETE FROM currencies WHERE code IN (
    'CZK', 'DKK', 'HUF', 'NOK', 'PLN', 'RON', 'RUB', 'UAH', 'ILS',
    'SAR', 'TRY', 'QAR', 'KWD', 'BHD', 'OMR', 'THB', 'VND', 'BDT',
    'PKR', 'LKR', 'NPR', 'HKD', 'TWD', 'EGP', 'NGN', 'KES', 'GHS',
    'MAD', 'ARS', 'CLP', 'COP', 'PEN', 'CRC', 'FJD'
);

-- Delete expanded countries
DELETE FROM countries WHERE id IN (
    'AT', 'BE', 'CZ', 'DK', 'FI', 'GR', 'HU', 'NO', 'PL', 'PT',
    'RO', 'RU', 'UA', 'IL', 'SA', 'TR', 'QA', 'KW', 'BH', 'OM',
    'TH', 'VN', 'BD', 'PK', 'LK', 'NP', 'MM', 'KH', 'HK', 'TW',
    'EG', 'NG', 'KE', 'GH', 'MA', 'TN', 'TZ', 'UG', 'ET',
    'AR', 'CL', 'CO', 'PE', 'VE', 'EC', 'UY', 'PA', 'CR', 'PR',
    'JM', 'DO', 'FJ'
);

-- Remove migration record
DELETE FROM schema_migrations WHERE version = 3;
