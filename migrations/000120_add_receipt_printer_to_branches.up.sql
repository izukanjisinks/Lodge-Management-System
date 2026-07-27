-- Each branch has its own physical receipt printer (e.g. Epson TM-T88VI over
-- Ethernet, raw ESC/POS port 9100). Config is stored per-branch so staff can
-- set it from the branch settings page instead of hardcoding an IP in code.

ALTER TABLE branches
    ADD COLUMN printer_ip   VARCHAR(45),  -- IPv4 or IPv6
    ADD COLUMN printer_port INT NOT NULL DEFAULT 9100,
    ADD COLUMN printer_name VARCHAR(255); -- optional friendly label, e.g. "Front Desk Receipt Printer"
