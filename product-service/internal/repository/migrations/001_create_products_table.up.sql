CREATE TABLE products (
                          id INTEGER PRIMARY KEY,
                          name TEXT NOT NULL,
                          description TEXT,
                          price REAL NOT NULL,
                          image_url TEXT,
                          stock INTEGER NOT NULL,
                          created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_products_id ON products(id);