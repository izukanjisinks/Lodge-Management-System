ALTER TABLE branches
    DROP COLUMN IF EXISTS printer_ip,
    DROP COLUMN IF EXISTS printer_port,
    DROP COLUMN IF EXISTS printer_name;
