-- +goose Up
CREATE TABLE event_registration_settings (
    event_id         uuid PRIMARY KEY REFERENCES events(id) ON DELETE CASCADE,
    default_mode     text NOT NULL DEFAULT 'NORMAL',
    queue_enabled    boolean NOT NULL DEFAULT false,
    ballot_enabled   boolean NOT NULL DEFAULT false,
    priority_enabled boolean NOT NULL DEFAULT false,
    waitlist_enabled boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT ers_mode_check CHECK (default_mode IN
        ('NORMAL','WAR_QUEUE','RANDOMIZED_QUEUE','HYBRID_QUEUE','BALLOT','INVITATION_ONLY','PRIORITY_ACCESS','WAITLIST_ONLY','CLOSED'))
);

CREATE TABLE category_registration_settings (
    category_id      uuid PRIMARY KEY REFERENCES event_categories(id) ON DELETE CASCADE,
    registration_mode text,
    override_enabled boolean NOT NULL DEFAULT false,
    created_at       timestamptz NOT NULL DEFAULT now(),
    updated_at       timestamptz NOT NULL DEFAULT now(),
    CONSTRAINT crs_mode_check CHECK (registration_mode IS NULL OR registration_mode IN
        ('NORMAL','WAR_QUEUE','RANDOMIZED_QUEUE','HYBRID_QUEUE','BALLOT','INVITATION_ONLY','PRIORITY_ACCESS','WAITLIST_ONLY','CLOSED'))
);

-- +goose Down
DROP TABLE category_registration_settings;
DROP TABLE event_registration_settings;
