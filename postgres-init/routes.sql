-- Insert sample routes for testing
INSERT INTO routes (path_pattern, service_name, method, target_url, auth_required, rate_limit, cache_ttl, enabled) VALUES
('/health', 'gateway', 'GET', '', false, 1000, 0, true),
('/routes', 'gateway', 'GET', '', false, 1000, 0, true),
('/api/users', 'user-service', 'ANY', 'http://user-service:3000', false, 100, 300, true),
('/api/products', 'product-service', 'ANY', 'http://product-service:3001', false, 100, 300, true),
('/api/public/status', 'status-service', 'GET', 'http://status-service:3002', false, 1000, 60, true);
