-- +goose Up

CREATE TABLE users (
    id                SERIAL PRIMARY KEY,
    public_id         UUID UNIQUE NOT NULL DEFAULT gen_random_uuid(),
    email             VARCHAR(255) UNIQUE NOT NULL,
    phone             VARCHAR(20),
    phone_verified    BOOLEAN NOT NULL DEFAULT FALSE,
    password_hash     VARCHAR(255),
    email_verified    BOOLEAN NOT NULL DEFAULT FALSE,
    email_verified_at TIMESTAMPTZ,
    otp_hash          VARCHAR(255),
    otp_expires_at    TIMESTAMPTZ,
    name              VARCHAR(100) NOT NULL,
    role              VARCHAR(20),
    job_id            INTEGER,
    region_id         INTEGER,
    status            VARCHAR(30) NOT NULL DEFAULT 'pending_documents'
        CHECK (status IN ('pending_documents','pending_review','active','rejected','suspended')),
    rejection_reason  TEXT,
    latitude          DECIMAL(10,7),
    longitude         DECIMAL(10,7),
    fcm_token         VARCHAR(500),
    created_at        TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_users_public_id ON users(public_id);
CREATE INDEX idx_users_email ON users(email);
CREATE INDEX idx_users_status ON users(status);

CREATE TABLE user_documents (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    document_type   VARCHAR(50) NOT NULL CHECK (document_type IN ('business_license','national_id','tax_card')),
    file_path       VARCHAR(500) NOT NULL,
    uploaded_at     TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    UNIQUE(user_id, document_type)
);

CREATE TABLE user_sessions (
    id              SERIAL PRIMARY KEY,
    user_id         INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    token_hash      VARCHAR(255) NOT NULL,
    is_current      BOOLEAN NOT NULL DEFAULT TRUE,
    device_info     TEXT,
    ip_address      VARCHAR(45),
    created_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    expires_at      TIMESTAMPTZ NOT NULL,
    last_used_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_user_sessions_user_id ON user_sessions(user_id);
CREATE INDEX idx_user_sessions_token_hash ON user_sessions(token_hash);

CREATE TABLE login_history (
    id          SERIAL PRIMARY KEY,
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    device_info TEXT,
    ip_address  VARCHAR(45),
    login_at    TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    success     BOOLEAN NOT NULL DEFAULT TRUE
);

CREATE INDEX idx_login_history_user_id ON login_history(user_id);

CREATE TABLE user_interests (
    user_id     INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    interest_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, interest_id)
);

-- +goose Down
DROP TABLE IF EXISTS user_interests;
DROP TABLE IF EXISTS login_history;
DROP TABLE IF EXISTS user_sessions;
DROP TABLE IF EXISTS user_documents;
DROP TABLE IF EXISTS users;
