# mcwebchunk


## db init

```sql
create table servers (
	id serial primary key,
	name text,
	ip text unique not null
);
create table dimensions (
	id serial primary key,
	server int references server(id) not null,
	name text not null,
);
create table chunks (
	id serial primary key,
	server int references server(id) not null,
	dim int references dimensions(id) not null,
	created_at timestamp default now(),
	x int not null,
	z int not null,
	data bytea not null,
)
```
