-- Drop webhook configurations table first to remove foreign key dependency
DROP TABLE IF EXISTS webhook_configurations CASCADE;

-- Drop webhook configuration audit
DROP TABLE IF EXISTS webhook_configuration_audit CASCADE;

-- Drop webhook health checks
DROP TABLE IF EXISTS webhook_health_checks CASCADE;

-- Drop webhook configuration templates
DROP TABLE IF EXISTS webhook_configuration_templates CASCADE;

-- Drop triggers
DROP TRIGGER IF EXISTS trigger_webhook_configurations_updated_at ON webhook_configurations;
DROP TRIGGER IF EXISTS trigger_webhook_configuration_templates_updated_at ON webhook_configuration_templates;

-- Drop function
DROP FUNCTION IF EXISTS update_webhook_configuration_updated_at();

-- Drop types
DROP TYPE IF EXISTS webhook_health_status;
DROP TYPE IF EXISTS webhook_auth_type;

-- Add back the webhook_config column to domains table
ALTER TABLE domains ADD COLUMN IF NOT EXISTS webhook_config JSONB;
