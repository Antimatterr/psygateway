-- Routes configuration
CREATE TABLE routes (
    id SERIAL PRIMARY KEY,
    path_pattern VARCHAR(255) NOT NULL,  -- "/api/users/*"
    service_name VARCHAR(100) NOT NULL,  -- "user-service" 
    method VARCHAR(10) DEFAULT 'ANY',    -- "GET", "POST", "ANY"
    target_url VARCHAR(500) NOT NULL,    -- "http://user-service:3000"
    auth_required BOOLEAN DEFAULT false,
    rate_limit INTEGER DEFAULT 100,
    cache_ttl INTEGER DEFAULT 0,         -- seconds, 0 = no cache
    enabled BOOLEAN DEFAULT true,
    created_at TIMESTAMP DEFAULT NOW()
);

-- Authentication rules
CREATE TABLE auth_rules (
    id SERIAL PRIMARY KEY,
    route_id INTEGER REFERENCES routes(id),
    required_roles TEXT[],               -- {"admin", "user"}
    api_key_required BOOLEAN DEFAULT false,
    jwt_required BOOLEAN DEFAULT false
);

-- API Keys
CREATE TABLE api_keys (
    id SERIAL PRIMARY KEY,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    name VARCHAR(100) NOT NULL,
    permissions TEXT[],
    rate_limit INTEGER DEFAULT 100,
    enabled BOOLEAN DEFAULT true,
    expires_at TIMESTAMP,
    created_at TIMESTAMP DEFAULT NOW()
);
