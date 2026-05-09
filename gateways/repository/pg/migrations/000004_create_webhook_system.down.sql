DROP TRIGGER IF EXISTS trigger_webhook_events_updated_at ON webhook_events;
DROP FUNCTION IF EXISTS update_webhook_event_updated_at();

DROP TABLE IF EXISTS webhook_events;

DROP TYPE IF EXISTS webhook_event_type;
DROP TYPE IF EXISTS webhook_event_status;