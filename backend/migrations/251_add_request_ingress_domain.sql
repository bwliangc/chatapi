ALTER TABLE usage_logs ADD COLUMN IF NOT EXISTS ingress_domain VARCHAR(253);
ALTER TABLE ops_error_logs ADD COLUMN IF NOT EXISTS ingress_domain VARCHAR(253);
ALTER TABLE batch_image_jobs ADD COLUMN IF NOT EXISTS ingress_domain VARCHAR(253);
