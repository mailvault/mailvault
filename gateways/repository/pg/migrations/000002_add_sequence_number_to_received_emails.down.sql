-- Remove the trigger and function
DROP TRIGGER IF EXISTS trigger_set_received_email_sequence ON received_emails;
DROP FUNCTION IF EXISTS set_received_email_sequence();

-- Remove the unique index
DROP INDEX IF EXISTS idx_email_sequence;

-- Remove the sequence_number column
ALTER TABLE received_emails DROP COLUMN IF EXISTS sequence_number;