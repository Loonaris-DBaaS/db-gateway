-- Tenant databases
CREATE DATABASE app_test1;
CREATE DATABASE app_test2;
CREATE DATABASE app_test3;

-- Connecting to app_test1 to create test table
\connect app_test1;
CREATE TABLE IF NOT EXISTS test_data (
    id SERIAL PRIMARY KEY,
    mode TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
INSERT INTO test_data (mode, message) VALUES ('rw', 'Hello from app_test1 (write)'), ('rw', 'Insert via RW connection');
GRANT ALL ON SCHEMA public TO PUBLIC;
GRANT ALL ON TABLE test_data TO PUBLIC;
GRANT USAGE, SELECT ON SEQUENCE test_data_id_seq TO PUBLIC;

\connect app_test2;
CREATE TABLE IF NOT EXISTS test_data (
    id SERIAL PRIMARY KEY,
    mode TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
INSERT INTO test_data (mode, message) VALUES ('rw', 'Hello from app_test2 (write)'), ('rw', 'Insert via RW connection');
GRANT ALL ON SCHEMA public TO PUBLIC;
GRANT ALL ON TABLE test_data TO PUBLIC;
GRANT USAGE, SELECT ON SEQUENCE test_data_id_seq TO PUBLIC;

\connect app_test3;
CREATE TABLE IF NOT EXISTS test_data (
    id SERIAL PRIMARY KEY,
    mode TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
INSERT INTO test_data (mode, message) VALUES ('rw', 'Hello from app_test3 (write)'), ('rw', 'Insert via RW connection');
GRANT ALL ON SCHEMA public TO PUBLIC;
GRANT ALL ON TABLE test_data TO PUBLIC;
GRANT USAGE, SELECT ON SEQUENCE test_data_id_seq TO PUBLIC;