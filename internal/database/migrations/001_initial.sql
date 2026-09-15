CREATE TABLE users (
    id text PRIMARY KEY,
    email text NOT NULL UNIQUE CHECK (email = lower(email)),
    name text NOT NULL CHECK (char_length(name) BETWEEN 2 AND 120),
    password_hash text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE sessions (
    token_hash text PRIMARY KEY,
    user_id text NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_sessions_user ON sessions(user_id);
CREATE INDEX idx_sessions_expiry ON sessions(expires_at);
CREATE TABLE events (
    id text PRIMARY KEY,
    owner_id text NOT NULL REFERENCES users(id),
    title text NOT NULL CHECK (char_length(title) BETWEEN 4 AND 120),
    description text NOT NULL CHECK (char_length(description) BETWEEN 20 AND 5000),
    category text NOT NULL CHECK (category IN ('Лекции','Карьера','Культура','Спорт','Сообщества')),
    location text NOT NULL CHECK (char_length(location) BETWEEN 3 AND 240),
    starts_at timestamptz NOT NULL,
    ends_at timestamptz NOT NULL,
    doors_at timestamptz NOT NULL,
    registration_ends_at timestamptz NOT NULL,
    capacity integer NOT NULL CHECK (capacity BETWEEN 1 AND 10000),
    status text NOT NULL DEFAULT 'draft' CHECK (status IN ('draft','published','cancelled')),
    color text NOT NULL DEFAULT 'blue' CHECK (color IN ('blue','violet','orange','green','pink')),
    version integer NOT NULL DEFAULT 1,
    created_at timestamptz NOT NULL DEFAULT now(),
    updated_at timestamptz NOT NULL DEFAULT now(),
    CHECK (doors_at <= starts_at AND starts_at < ends_at AND registration_ends_at <= ends_at),
    CHECK (ends_at - doors_at <= interval '14 days')
);
CREATE INDEX idx_events_public ON events(status, starts_at);
CREATE INDEX idx_events_owner ON events(owner_id, starts_at);
CREATE TABLE event_staff (
    id text PRIMARY KEY,
    event_id text NOT NULL REFERENCES events(id),
    label text NOT NULL CHECK (char_length(label) BETWEEN 2 AND 120),
    role text NOT NULL CHECK (role IN ('controller','organizer')),
    token_hash text NOT NULL UNIQUE,
    user_id text REFERENCES users(id),
    invited_by text NOT NULL REFERENCES users(id),
    expires_at timestamptz NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now(),
    accepted_at timestamptz,
    revoked_at timestamptz,
    CHECK ((accepted_at IS NULL) = (user_id IS NULL))
);
CREATE UNIQUE INDEX idx_staff_active_user ON event_staff(event_id,user_id) WHERE user_id IS NOT NULL AND revoked_at IS NULL;
CREATE INDEX idx_staff_user ON event_staff(user_id,event_id);
CREATE INDEX idx_staff_event ON event_staff(event_id,created_at);
CREATE TABLE passes (
    id text PRIMARY KEY,
    event_id text NOT NULL REFERENCES events(id),
    holder_id text REFERENCES users(id),
    holder_name text NOT NULL CHECK (char_length(holder_name) BETWEEN 2 AND 120),
    code_hash text NOT NULL UNIQUE,
    status text NOT NULL DEFAULT 'active' CHECK (status IN ('active','redeemed','revoked')),
    issued_at timestamptz NOT NULL DEFAULT now(),
    issued_by text NOT NULL REFERENCES users(id),
    request_id text NOT NULL,
    request_hash text NOT NULL,
    redeemed_at timestamptz,
    redeemed_by text REFERENCES users(id),
    revoked_at timestamptz,
    revoke_reason text,
    UNIQUE(issued_by,request_id),
    CHECK ((status = 'redeemed' AND redeemed_at IS NOT NULL AND redeemed_by IS NOT NULL) OR (status <> 'redeemed' AND redeemed_at IS NULL AND redeemed_by IS NULL)),
    CHECK ((status = 'revoked') = (revoked_at IS NOT NULL))
);
CREATE UNIQUE INDEX idx_passes_holder_active ON passes(event_id,holder_id) WHERE holder_id IS NOT NULL AND status <> 'revoked';
CREATE INDEX idx_passes_event ON passes(event_id,status,issued_at);
CREATE INDEX idx_passes_holder ON passes(holder_id,issued_at);
CREATE TABLE redemptions (
    actor_id text NOT NULL REFERENCES users(id),
    request_id text NOT NULL,
    pass_id text NOT NULL UNIQUE REFERENCES passes(id),
    event_id text NOT NULL REFERENCES events(id),
    redeemed_at timestamptz NOT NULL DEFAULT now(),
    PRIMARY KEY(actor_id,request_id)
);
CREATE TABLE audit_log (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id text NOT NULL REFERENCES events(id),
    actor_id text NOT NULL REFERENCES users(id),
    action text NOT NULL,
    target_id text,
    details jsonb NOT NULL DEFAULT '{}',
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_event_time ON audit_log(event_id,id DESC);
CREATE TABLE admission_attempts (
    id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    event_id text NOT NULL REFERENCES events(id),
    actor_id text NOT NULL REFERENCES users(id),
    pass_id text REFERENCES passes(id),
    result text NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX idx_admission_event_time ON admission_attempts(event_id,id DESC);
CREATE TABLE rate_limits (
    key text PRIMARY KEY,
    window_start timestamptz NOT NULL,
    hits integer NOT NULL
);
CREATE INDEX idx_rate_limits_window ON rate_limits(window_start);
