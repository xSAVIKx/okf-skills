-- Initialize sample e-commerce database for PostgreSQL

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    username VARCHAR(50) NOT NULL UNIQUE,
    email VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE users IS 'Registered users of the store';
COMMENT ON COLUMN users.id IS 'Unique identifier for the user';
COMMENT ON COLUMN users.username IS 'Unique login username';
COMMENT ON COLUMN users.email IS 'Contact email address';
COMMENT ON COLUMN users.created_at IS 'Timestamp of account creation';

-- Products table
CREATE TABLE IF NOT EXISTS products (
    id SERIAL PRIMARY KEY,
    name VARCHAR(100) NOT NULL,
    price DECIMAL(10, 2) NOT NULL,
    stock_quantity INT DEFAULT 0
);

COMMENT ON TABLE products IS 'Catalog of products available for purchase';
COMMENT ON COLUMN products.id IS 'Unique identifier for the product';
COMMENT ON COLUMN products.name IS 'Display name of the product';
COMMENT ON COLUMN products.price IS 'Unit price in USD';
COMMENT ON COLUMN products.stock_quantity IS 'Number of units available in stock';

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    user_id INT NOT NULL REFERENCES users(id),
    total_amount DECIMAL(10, 2) NOT NULL,
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

COMMENT ON TABLE orders IS 'Purchase transactions history';
COMMENT ON COLUMN orders.id IS 'Unique identifier for the order';
COMMENT ON COLUMN orders.user_id IS 'Reference key to the ordering user';
COMMENT ON COLUMN orders.total_amount IS 'Total cost of the order';
COMMENT ON COLUMN orders.order_date IS 'Timestamp when the order was placed';
