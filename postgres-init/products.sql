CREATE TABLE products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(255) NOT NULL,
    description TEXT,
    price NUMERIC(10, 2) NOT NULL,          -- e.g., 99999999.99 max
    stock_quantity INTEGER DEFAULT 0,       -- current stock
    category VARCHAR(100),                  -- optional category tag
    status VARCHAR(20) DEFAULT 'active',    -- 'active', 'inactive', 'discontinued'
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE TRIGGER update_products_timestamp
BEFORE UPDATE ON products
FOR EACH ROW
EXECUTE FUNCTION update_timestamp();


INSERT INTO products (name, description, price, stock_quantity, category, status)
VALUES
('Wireless Mouse', 'Ergonomic wireless mouse with 2.4GHz connection', 25.99, 150, 'Electronics', 'active'),
('Mechanical Keyboard', 'RGB backlit mechanical keyboard with blue switches', 79.99, 75, 'Electronics', 'active'),
('Office Chair', 'Adjustable ergonomic office chair', 199.50, 40, 'Furniture', 'active'),
('Coffee Mug', 'Ceramic mug 350ml capacity', 9.99, 200, 'Kitchen', 'active'),
('Gaming Monitor', '27-inch QHD monitor with 144Hz refresh rate', 299.99, 30, 'Electronics', 'active');
