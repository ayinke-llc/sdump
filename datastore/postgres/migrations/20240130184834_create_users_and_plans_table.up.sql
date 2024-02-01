CREATE TABLE IF NOT EXISTS plans(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    human_readable_name VARCHAR (200) NOT NULL,

    allowances jsonb NOT NULL DEFAULT '{}'::jsonb,

    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

CREATE TABLE IF NOT EXISTS users(
    id uuid PRIMARY KEY DEFAULT uuid_generate_v4(),
    ssh_finger_print VARCHAR (200) UNIQUE NOT NULL,
    is_banned BOOLEAN DEFAULT FALSE NOT NULL,
    plan_id uuid NOT NULL REFERENCES plans(id),

    created_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP WITH TIME ZONE
);

ALTER TABLE urls ADD user_id uuid NOT NULL REFERENCES users(id);
