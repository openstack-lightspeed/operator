#!/bin/bash

set -e

cat /var/lib/pgsql/data/userdata/postgresql.conf

echo "attempting to create llama-stack database and pg_trgm extension if they do not exist"

_psql () { psql --set ON_ERROR_STOP=1 "$@" ; }

# Create database for llama-stack conversation storage
DB_NAME="llamastack"

echo "SELECT 'CREATE DATABASE $DB_NAME' WHERE NOT EXISTS (SELECT FROM pg_database WHERE datname = '$DB_NAME')\gexec" | _psql -d $POSTGRESQL_DATABASE

# Create pg_trgm extension in default database (for OpenStack Lightspeed conversation cache)
echo "CREATE EXTENSION IF NOT EXISTS pg_trgm;" | _psql -d $POSTGRESQL_DATABASE

# Create pg_trgm extension in llama-stack database (for text search if needed)
echo "CREATE EXTENSION IF NOT EXISTS pg_trgm;" | _psql -d $DB_NAME

# Create schemas for isolating different components' data
echo "CREATE SCHEMA IF NOT EXISTS lcore;" | _psql -d $POSTGRESQL_DATABASE
echo "CREATE SCHEMA IF NOT EXISTS quota;" | _psql -d $POSTGRESQL_DATABASE
echo "CREATE SCHEMA IF NOT EXISTS conversation_cache;" | _psql -d $POSTGRESQL_DATABASE
