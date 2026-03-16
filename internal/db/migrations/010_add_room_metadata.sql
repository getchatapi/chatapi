-- Add metadata column to rooms for application-level context.
-- Developers can store arbitrary JSON (listing_id, order_id, etc.) on a room
-- so their app can identify which conversation belongs to which resource.
ALTER TABLE rooms ADD COLUMN metadata JSON NULL;
