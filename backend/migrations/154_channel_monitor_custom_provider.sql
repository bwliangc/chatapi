-- Migration: 154_channel_monitor_custom_provider
-- Allow channel monitor configs and request templates to use a generic OpenAI-compatible custom provider.

DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'channel_monitors_provider_check'
          AND table_name = 'channel_monitors'
    ) THEN
        ALTER TABLE channel_monitors
            DROP CONSTRAINT channel_monitors_provider_check;
    END IF;

    ALTER TABLE channel_monitors
        ADD CONSTRAINT channel_monitors_provider_check
        CHECK (provider IN ('openai', 'anthropic', 'gemini', 'custom'));

    IF EXISTS (
        SELECT 1 FROM information_schema.table_constraints
        WHERE constraint_name = 'channel_monitor_request_templates_provider_check'
          AND table_name = 'channel_monitor_request_templates'
    ) THEN
        ALTER TABLE channel_monitor_request_templates
            DROP CONSTRAINT channel_monitor_request_templates_provider_check;
    END IF;

    ALTER TABLE channel_monitor_request_templates
        ADD CONSTRAINT channel_monitor_request_templates_provider_check
        CHECK (provider IN ('openai', 'anthropic', 'gemini', 'custom'));
END $$;
