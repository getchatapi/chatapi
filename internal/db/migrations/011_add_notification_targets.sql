-- Add targets column to notifications for scoped delivery
ALTER TABLE notifications ADD COLUMN targets JSON NULL;
