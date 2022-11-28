#!/bin/bash
set -e

psql -v ON_ERROR_STOP=1 --username "$POSTGRES_USER" --dbname "$POSTGRES_DB" <<-EOSQL
	CREATE TABLE public.worlds (
		name text NOT NULL PRIMARY KEY,
		alias text,
		ip text,
		created_at timestamp DEFAULT now(),
		data json
	);
	CREATE TABLE public.dimensions (
		id SERIAL PRIMARY KEY,
		world text NOT NULL REFERENCES worlds (name),
		name text NOT NULL,
		created_at timestamp DEFAULT now(),
		data json,
		UNIQUE (world, name)
	);
	CREATE TABLE public.chunks (
		id SERIAL PRIMARY KEY,
		dim integer NOT NULL REFERENCES dimensions (id),
		created_at timestamp DEFAULT now(),
		x integer NOT NULL,
		z integer NOT NULL,
		data bytea NOT NULL
	);
EOSQL

