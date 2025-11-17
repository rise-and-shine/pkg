# Outbox Pattern Migration Templates

This directory contains **reference SQL migration templates** for services implementing the outbox pattern.

## ⚠️ Important: These are Templates, Not Active Migrations

These files are:
- ✅ Reference implementations
- ✅ Documentation of the required schema
- ✅ Starting points for your service migrations
- ❌ **NOT** intended to be run directly by migration tools

## 📋 Usage Instructions

### Step 1: Copy to Your Service

```bash
# Copy the outbox migration template to your service
cp pkg/outbox/migrations/001_enhanced_outbox.sql your-service/migrations/001_create_outbox.sql
```

### Step 2: Customize for Your Service

Each service should customize the migration based on:

1. **Service Volume**: High-volume services need partitioning
2. **Query Patterns**: Add indexes for your specific queries
3. **Retention**: Adjust cleanup policies
4. **Database Type**: Adapt for PostgreSQL, MySQL, etc.

### Step 3: Run with Your Migration Tool

Use your service's migration tool:
- `golang-migrate`
- `goose`
- `atlas`
- Custom migration runner

---

## 📄 Available Templates

### `001_enhanced_outbox.sql`

Creates the core outbox table with:
- ✅ Event storage (id, aggregate_id, event_type, event_data)
- ✅ Publishing metadata (topic, status, sent_at)
- ✅ Retry logic (retry_count, error)
- ✅ Timestamps (created_at, updated_at)
- ✅ Basic indexes for performance

**Columns:**
```sql
id              BIGSERIAL PRIMARY KEY
aggregate_id    VARCHAR(255)  -- Links event to domain entity
event_type      VARCHAR(255)  -- e.g., "order.created"
event_data      JSONB         -- Full event payload
topic           VARCHAR(255)  -- Kafka/message broker topic
status          VARCHAR(50)   -- pending, sent, failed
retry_count     INT           -- Number of retry attempts
created_at      TIMESTAMP
updated_at      TIMESTAMP
sent_at         TIMESTAMP     -- When successfully published
error           TEXT          -- Last error message
```

---

## 🔧 Service-Specific Customizations

### Example 1: High-Volume Service

```sql
-- Add partitioning for better performance
CREATE TABLE outbox (
    -- ... columns ...
    created_at TIMESTAMP NOT NULL
) PARTITION BY RANGE (created_at);

-- Create monthly partitions
CREATE TABLE outbox_2025_11 PARTITION OF outbox
    FOR VALUES FROM ('2025-11-01') TO ('2025-12-01');

CREATE TABLE outbox_2025_12 PARTITION OF outbox
    FOR VALUES FROM ('2025-12-01') TO ('2026-01-01');
```

### Example 2: Multi-Tenant Service

```sql
-- Add tenant_id for isolation
ALTER TABLE outbox ADD COLUMN tenant_id VARCHAR(255);

CREATE INDEX idx_outbox_tenant_status ON outbox(tenant_id, status) 
WHERE status = 'pending';
```

### Example 3: Priority Events

```sql
-- Add priority for event ordering
ALTER TABLE outbox ADD COLUMN priority INT DEFAULT 0;

CREATE INDEX idx_outbox_pending_priority ON outbox(priority DESC, created_at) 
WHERE status = 'pending';
```

### Example 4: Service-Specific Indexes

```sql
-- Index for your service's specific query patterns
CREATE INDEX idx_outbox_service_pattern ON outbox(status, topic, created_at)
WHERE status IN ('pending', 'failed');

-- Partial index for cleanup queries
CREATE INDEX idx_outbox_cleanup ON outbox(created_at)
WHERE status = 'sent' AND created_at < NOW() - INTERVAL '7 days';
```

---

## 🏗️ Migration Structure Per Service

Each service should have its own migrations:

```
your-service/
  migrations/
    001_create_outbox.sql       ← Copy from this template
    002_create_orders.sql       ← Service domain tables
    003_create_order_items.sql  ← Service domain tables
    004_add_outbox_indexes.sql  ← Service-specific optimizations
```

---

## 📚 Related Documentation

- **Outbox Pattern Overview**: `../README.md`
- **Integration Guide**: `../USAGE_GUIDE.md`
- **Schema Design**: See SQL file comments

---

## 🤝 Contributing

If you find improvements to the base schema:

1. Update the template in `pkg/outbox/migrations/`
2. Document the change in this README
3. Submit a PR with rationale

**Note**: Template changes should be backward compatible!

---

## ❓ FAQ

### Q: Should I use these migrations directly?

**A: No.** Copy them to your service and customize.

### Q: What if I need a different table name?

**A: Rename it!** The outbox package is configurable. Just update the repository initialization:

```go
repo := outbox.NewRepository(db, outbox.WithTableName("my_events"))
```

### Q: Can I add columns?

**A: Yes!** Add service-specific metadata as needed. The package uses these core columns:
- `id`, `aggregate_id`, `event_type`, `event_data`
- `topic`, `status`, `retry_count`
- `created_at`, `updated_at`, `sent_at`, `error`

### Q: What about database compatibility?

**A: Adapt as needed.** The template is PostgreSQL-focused but concepts apply to:
- MySQL/MariaDB
- SQLite (for testing)
- CockroachDB
- YugabyteDB

---

## 🔄 Version History

- **v1.0**: Initial enhanced outbox schema with retry logic
- **v1.1**: Added indexes for common query patterns
- **v1.2**: Documentation improvements

---

**Remember**: Each service owns its schema. Use these as templates, not dependencies!
