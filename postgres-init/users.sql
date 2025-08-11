CREATE TABLE users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) UNIQUE NOT NULL,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    role VARCHAR(50) DEFAULT 'user',       -- 'user', 'admin'
    status VARCHAR(20) DEFAULT 'active',   -- 'active', 'inactive', 'banned'
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);


CREATE OR REPLACE FUNCTION update_timestamp()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;

CREATE TRIGGER update_users_timestamp
BEFORE UPDATE ON users
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();


INSERT INTO users (username, email, password_hash, role, status)
VALUES 
('admin', 'admin@example.com', 'hashedpassword123', 'admin', 'active'),
('john_doe', 'john@example.com', 'hashedpassword456', 'user', 'active'),
('jane_doe', 'jane@example.com', 'hashedpassword789', 'user', 'inactive');
