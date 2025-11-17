-- Enhanced Outbox Schema Migration
-- This migration adds new fields based on the article suggestions

-- First, let's create the enhanced outbox_events table with all new features
CREATE TABLE IF NOT EXISTS outbox_events (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    aggregate_type VARCHAR(255) NOT NULL,
    aggregate_id VARCHAR(255) NOT NULL,
    event_type VARCHAR(255) NOT NULL,
    event_version VARCHAR(50) NOT NULL DEFAULT 'v1',
    event_data JSONB NOT NULL,
    headers JSONB NOT NULL DEFAULT '{}',
    topic VARCHAR(255) NOT NULL,
    partition_key VARCHAR(255),
    dedup_key VARCHAR(255),
    user_id VARCHAR(255),
    trace_id VARCHAR(255),
    status VARCHAR(50) NOT NULL DEFAULT 'pending',
    attempts INTEGER NOT NULL DEFAULT 0,
    retry_count INTEGER NOT NULL DEFAULT 0,
    next_retry_at TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    available_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    published_at TIMESTAMP WITH TIME ZONE,
    error_message TEXT,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    seq BIGSERIAL
);

-- Create unique constraint on dedup_key to prevent duplicates
CREATE UNIQUE INDEX IF NOT EXISTS idx_outbox_events_dedup_key 
ON outbox_events (dedup_key) WHERE dedup_key IS NOT NULL;

-- Create index for efficient querying of available events
CREATE INDEX IF NOT EXISTS idx_outbox_events_available 
ON outbox_events (status, available_at) WHERE status = 'pending';

-- Create index for aggregate queries
CREATE INDEX IF NOT EXISTS idx_outbox_events_aggregate 
ON outbox_events (aggregate_type, aggregate_id);

-- Create index for status and created_at for efficient polling
CREATE INDEX IF NOT EXISTS idx_outbox_events_status_created 
ON outbox_events (status, created_at);

-- Create index for cleanup operations
CREATE INDEX IF NOT EXISTS idx_outbox_events_cleanup 
ON outbox_events (status, published_at) WHERE status = 'sent';

-- Create enhanced outbox_offset table with UUID support
CREATE TABLE IF NOT EXISTS outbox_offset (
    service_name VARCHAR(255) PRIMARY KEY,
    last_event_id UUID,
    last_processed TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP
);

-- Create foreign key constraint for offset table
ALTER TABLE outbox_offset 
ADD CONSTRAINT fk_outbox_offset_event 
FOREIGN KEY (last_event_id) REFERENCES outbox_events(id) ON DELETE SET NULL;

-- Add comments for documentation
COMMENT ON TABLE outbox_events IS 'Enhanced outbox pattern implementation with deduplication, retry logic, and delayed processing';
COMMENT ON COLUMN outbox_events.aggregate_type IS 'Type of the aggregate (e.g., User, Order, Product)';
COMMENT ON COLUMN outbox_events.dedup_key IS 'Unique key to prevent duplicate events';
COMMENT ON COLUMN outbox_events.available_at IS 'Timestamp when event becomes available for processing (supports delayed processing and exponential backoff)';
COMMENT ON COLUMN outbox_events.headers IS 'Additional metadata for the event';
COMMENT ON COLUMN outbox_events.seq IS 'Sequential number for ordering events within same aggregate';

-- Trigger to automatically update updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = CURRENT_TIMESTAMP;
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_outbox_events_updated_at 
    BEFORE UPDATE ON outbox_events 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

CREATE TRIGGER update_outbox_offset_updated_at 
    BEFORE UPDATE ON outbox_offset 
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Optional: Create a view for pending events ready for processing
CREATE OR REPLACE VIEW pending_events AS
SELECT *
FROM outbox_events
WHERE status = 'pending' 
AND available_at <= CURRENT_TIMESTAMP
ORDER BY created_at ASC;

-- Optional: Create a view for failed events ready for retry
CREATE OR REPLACE VIEW retry_events AS
SELECT *
FROM outbox_events
WHERE status = 'failed' 
AND available_at <= CURRENT_TIMESTAMP
ORDER BY created_at ASC;

-- Statistics for monitoring
CREATE OR REPLACE VIEW outbox_stats AS
SELECT 
    status,
    COUNT(*) as event_count,
    MIN(created_at) as oldest_event,
    MAX(created_at) as newest_event,
    AVG(attempts) as avg_attempts
FROM outbox_events
GROUP BY status;