CREATE USER sk_live_tenant2;
ALTER USER sk_live_tenant2 CREATEDB;
GRANT ALL PRIVILEGES ON DATABASE db2 TO sk_live_tenant2;
