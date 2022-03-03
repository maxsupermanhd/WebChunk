#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	CREATE TABLE public.servers (
		id SERIAL PRIMARY KEY,
		name text NOT NULL UNIQUE,
		ip text NOT NULL UNIQUE
	);
	CREATE TABLE public.dimensions (
		id SERIAL PRIMARY KEY,
		server integer NOT NULL REFERENCES servers (id),
		name text NOT NULL UNIQUE,
		alias text UNIQUE
	);
	CREATE TABLE public.chunks (
		id SERIAL PRIMARY KEY,
		server integer NOT NULL REFERENCES servers (id),
		dim integer NOT NULL REFERENCES dimensions (id),
		created_at timestamp DEFAULT now(),
		x integer NOT NULL,
		z integer NOT NULL,
		data bytea NOT NULL
	);
EOSQL

