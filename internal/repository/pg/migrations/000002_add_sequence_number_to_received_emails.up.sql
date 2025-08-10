-- Add sequence number column to received_emails table
ALTER TABLE received_emails ADD COLUMN sequence_number INTEGER;

-- Create function to automatically set sequence number
CREATE OR REPLACE FUNCTION set_received_email_sequence()
RETURNS TRIGGER AS $$
BEGIN
    -- Get the next sequence number for this email address
    SELECT COALESCE(MAX(sequence_number), 0) + 1
    INTO NEW.sequence_number
    FROM received_emails
    WHERE email_address_id = NEW.email_address_id;
    
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

-- Create trigger to auto-set sequence number on insert
CREATE TRIGGER trigger_set_received_email_sequence
    BEFORE INSERT ON received_emails
    FOR EACH ROW
    EXECUTE FUNCTION set_received_email_sequence();

-- Create unique constraint on email_address_id + sequence_number
CREATE UNIQUE INDEX idx_email_sequence ON received_emails(email_address_id, sequence_number);

-- Backfill sequence numbers for existing emails
DO $$
DECLARE
    email_addr_rec RECORD;
    email_rec RECORD;
    seq_num INTEGER;
BEGIN
    -- For each email address, assign sequence numbers to existing emails
    FOR email_addr_rec IN SELECT id FROM email_addresses LOOP
        seq_num := 1;
        FOR email_rec IN 
            SELECT id FROM received_emails 
            WHERE email_address_id = email_addr_rec.id 
            ORDER BY received_at ASC
        LOOP
            UPDATE received_emails 
            SET sequence_number = seq_num 
            WHERE id = email_rec.id;
            seq_num := seq_num + 1;
        END LOOP;
    END LOOP;
END $$;

-- Make sequence_number NOT NULL after backfill
ALTER TABLE received_emails ALTER COLUMN sequence_number SET NOT NULL;