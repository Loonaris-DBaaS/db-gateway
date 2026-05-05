CREATE USER sk_live_tenant1;
ALTER USER sk_live_tenant1 CREATEDB;
GRANT ALL PRIVILEGES ON DATABASE db1 TO sk_live_tenant1;
