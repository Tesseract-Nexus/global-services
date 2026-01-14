-- Migration: Create exchange_rates table for currency conversion
-- Description: Stores exchange rates fetched from Frankfurter.app API

-- Create exchange_rates table
CREATE TABLE IF NOT EXISTS exchange_rates (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    base_currency VARCHAR(3) NOT NULL,
    target_currency VARCHAR(3) NOT NULL,
    rate DECIMAL(20, 10) NOT NULL,
    fetched_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT NOW(),
    deleted_at TIMESTAMP WITH TIME ZONE,
    CONSTRAINT exchange_rates_unique_pair UNIQUE(base_currency, target_currency)
);

-- Create indexes for efficient queries
CREATE INDEX IF NOT EXISTS idx_exchange_rates_currencies
    ON exchange_rates(base_currency, target_currency)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_exchange_rates_base_currency
    ON exchange_rates(base_currency)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_exchange_rates_fetched_at
    ON exchange_rates(fetched_at DESC)
    WHERE deleted_at IS NULL;

-- Create trigger for updated_at
CREATE OR REPLACE FUNCTION update_exchange_rates_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

DROP TRIGGER IF EXISTS trigger_exchange_rates_updated_at ON exchange_rates;

CREATE TRIGGER trigger_exchange_rates_updated_at
    BEFORE UPDATE ON exchange_rates
    FOR EACH ROW
    EXECUTE FUNCTION update_exchange_rates_updated_at();

-- Add comments for documentation
COMMENT ON TABLE exchange_rates IS 'Stores currency exchange rates from Frankfurter.app API';
COMMENT ON COLUMN exchange_rates.base_currency IS 'ISO 4217 currency code for the base currency (e.g., USD, EUR)';
COMMENT ON COLUMN exchange_rates.target_currency IS 'ISO 4217 currency code for the target currency';
COMMENT ON COLUMN exchange_rates.rate IS 'Exchange rate: 1 unit of base_currency = rate units of target_currency';
COMMENT ON COLUMN exchange_rates.fetched_at IS 'Timestamp when the rate was fetched from the external API';
