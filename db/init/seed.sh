#!/bin/bash

PGPASSWORD=$POSTGRES_PASSWORD psql -U $POSTGRES_USER -d $POSTGRES_DB << EOF
INSERT INTO users (username, password_hash, role) 
VALUES ('$DEFAULT_TESTUSER_USERNAME', crypt('$DEFAULT_TESTUSER_PASSWORD', gen_salt('bf')), 'user')
ON CONFLICT (username) DO UPDATE SET password_hash = EXCLUDED.password_hash;

INSERT INTO users (username, password_hash, role) 
VALUES ('$DEFAULT_ADMIN_USERNAME', crypt('$DEFAULT_ADMIN_PASSWORD', gen_salt('bf')), 'admin')
ON CONFLICT (username) DO UPDATE SET password_hash = EXCLUDED.password_hash;
EOF
