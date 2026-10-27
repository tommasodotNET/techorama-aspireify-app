CREATE TABLE IF NOT EXISTS products (
    id uuid PRIMARY KEY,
    name varchar(60) NOT NULL,
    description varchar(180) NOT NULL,
    icon varchar(20) NOT NULL,
    price numeric(8,2) NOT NULL CHECK (price > 0),
    stock integer NOT NULL CHECK (stock >= 0),
    active boolean NOT NULL DEFAULT true
);
CREATE TABLE IF NOT EXISTS orders (
    id uuid PRIMARY KEY,
    product_id uuid NOT NULL REFERENCES products(id),
    product_name varchar(60) NOT NULL,
    total numeric(8,2) NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE TABLE IF NOT EXISTS seed_history (version integer PRIMARY KEY);

-- A durable seed marker prevents removed fruit from returning on restart.
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM seed_history WHERE version = 1) THEN
        INSERT INTO products (id, name, description, icon, price, stock) VALUES
          ('10000000-0000-0000-0000-000000000001', 'Bendy bananas', 'A bunch of sunshine. Perfect for a very long lunch.', 'banana', 3.50, 24),
          ('10000000-0000-0000-0000-000000000002', 'Sky-high apples', 'Crisp, sweet, and picked from the highest branches.', 'apple', 2.25, 18),
          ('10000000-0000-0000-0000-000000000003', 'Savanna oranges', 'Fresh citrus for your next watering-hole picnic.', 'orange', 2.75, 16),
          ('10000000-0000-0000-0000-000000000004', 'Melon of the month', 'Big fruit. Big neck energy. One whole juicy melon.', 'melon', 6.00, 8);
        INSERT INTO seed_history VALUES (1);
    END IF;
END $$;
