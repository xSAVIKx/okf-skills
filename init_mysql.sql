-- Initialize sample e-commerce database for MySQL
CREATE DATABASE IF NOT EXISTS ecommerce;
USE ecommerce;

-- Users table
CREATE TABLE IF NOT EXISTS users (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Unique identifier for the user',
    username VARCHAR(50) NOT NULL UNIQUE COMMENT 'Unique login username',
    email VARCHAR(100) NOT NULL COMMENT 'Contact email address',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Timestamp of account creation'
) COMMENT='Registered users of the store';

-- Products table
CREATE TABLE IF NOT EXISTS products (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Unique identifier for the product',
    name VARCHAR(100) NOT NULL COMMENT 'Display name of the product',
    price DECIMAL(10, 2) NOT NULL COMMENT 'Unit price in USD',
    stock_quantity INT DEFAULT 0 COMMENT 'Number of units available in stock'
) COMMENT='Catalog of products available for purchase';

-- Orders table
CREATE TABLE IF NOT EXISTS orders (
    id INT AUTO_INCREMENT PRIMARY KEY COMMENT 'Unique identifier for the order',
    user_id INT NOT NULL COMMENT 'Reference key to the ordering user',
    total_amount DECIMAL(10, 2) NOT NULL COMMENT 'Total cost of the order',
    order_date TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Timestamp when the order was placed',
    FOREIGN KEY (user_id) REFERENCES users(id)
) COMMENT='Purchase transactions history';
