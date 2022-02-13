create extension if not exists pg_stat_statements;

CREATE TABLE public.chunks (
    id integer NOT NULL,
    server integer NOT NULL,
    dim integer NOT NULL,
    created_at timestamp without time zone DEFAULT now(),
    x integer NOT NULL,
    z integer NOT NULL,
    data bytea NOT NULL
);

ALTER TABLE public.chunks OWNER TO webchunk;

CREATE SEQUENCE public.chunks_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER TABLE public.chunks_id_seq OWNER TO webchunk;

ALTER SEQUENCE public.chunks_id_seq OWNED BY public.chunks.id;

CREATE TABLE public.dimensions (
    id integer NOT NULL,
    server integer NOT NULL,
    name text NOT NULL,
    alias text
);

ALTER TABLE public.dimensions OWNER TO webchunk;

CREATE SEQUENCE public.dimensions_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER TABLE public.dimensions_id_seq OWNER TO webchunk;

ALTER SEQUENCE public.dimensions_id_seq OWNED BY public.dimensions.id;

CREATE TABLE public.servers (
    id integer NOT NULL,
    name text,
    ip text NOT NULL
);

ALTER TABLE public.servers OWNER TO webchunk;

CREATE SEQUENCE public.servers_id_seq
    AS integer
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;

ALTER TABLE public.servers_id_seq OWNER TO webchunk;

ALTER SEQUENCE public.servers_id_seq OWNED BY public.servers.id;

ALTER TABLE ONLY public.chunks ALTER COLUMN id SET DEFAULT nextval('public.chunks_id_seq'::regclass);

ALTER TABLE ONLY public.dimensions ALTER COLUMN id SET DEFAULT nextval('public.dimensions_id_seq'::regclass);

ALTER TABLE ONLY public.servers ALTER COLUMN id SET DEFAULT nextval('public.servers_id_seq'::regclass);

ALTER TABLE ONLY public.chunks
    ADD CONSTRAINT chunks_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.dimensions
    ADD CONSTRAINT dimensions_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.servers
    ADD CONSTRAINT servers_ip_key UNIQUE (ip);

ALTER TABLE ONLY public.servers
    ADD CONSTRAINT servers_pkey PRIMARY KEY (id);

ALTER TABLE ONLY public.chunks
    ADD CONSTRAINT chunks_dim_fkey FOREIGN KEY (dim) REFERENCES public.dimensions(id);

ALTER TABLE ONLY public.chunks
    ADD CONSTRAINT chunks_server_fkey FOREIGN KEY (server) REFERENCES public.servers(id);

ALTER TABLE ONLY public.dimensions
    ADD CONSTRAINT dimensions_server_fkey FOREIGN KEY (server) REFERENCES public.servers(id);
