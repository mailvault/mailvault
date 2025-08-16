-- Add auto_create_address column to domains table
ALTER TABLE domains ADD COLUMN auto_create_address BOOLEAN DEFAULT false;

-- Add comment for documentation
COMMENT ON COLUMN domains.auto_create_address IS 'Whether to automatically create email addresses when receiving emails to non-existent addresses';