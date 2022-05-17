module github.com/maxsupermanhd/WebChunk

go 1.18

replace github.com/maxsupermanhd/WebChunk/chunkStorage => ./chunkStorage

replace github.com/maxsupermanhd/WebChunk/filesystemChunkStorage => ./filesystemChunkStorage

replace github.com/maxsupermanhd/WebChunk/postgresChunkStorage => ./postgresChunkStorage

replace github.com/maxsupermanhd/WebChunk/viewer => ./viewer

replace github.com/maxsupermanhd/WebChunk/proxy => ./proxy

replace github.com/maxsupermanhd/WebChunk/cmd/auth => ./cmd/auth

require (
	github.com/Tnze/go-mc v1.18.2-0.20220517073825-ad0ea1db6810
	github.com/davecgh/go-spew v1.1.1
	github.com/dustin/go-humanize v1.0.0
	github.com/fsnotify/fsnotify v1.5.4
	github.com/georgysavva/scany v1.0.0
	github.com/gorilla/handlers v1.5.1
	github.com/gorilla/mux v1.8.0
	github.com/jackc/pgx/v4 v4.16.1
	github.com/joho/godotenv v1.4.0
	github.com/natefinch/lumberjack v2.0.0+incompatible
	github.com/nfnt/resize v0.0.0-20180221191011-83c6a9932646
	github.com/shirou/gopsutil v3.21.11+incompatible
)

require github.com/yusufpapurcu/wmi v1.2.2 // indirect

require (
	github.com/felixge/httpsnoop v1.0.3 // indirect
	github.com/go-ole/go-ole v1.2.6 // indirect
	github.com/google/uuid v1.3.0
	github.com/jackc/chunkreader/v2 v2.0.1 // indirect
	github.com/jackc/pgconn v1.12.1 // indirect
	github.com/jackc/pgio v1.0.0 // indirect
	github.com/jackc/pgpassfile v1.0.0 // indirect
	github.com/jackc/pgproto3/v2 v2.3.0 // indirect
	github.com/jackc/pgservicefile v0.0.0-20200714003250-2b9c44734f2b // indirect
	github.com/jackc/pgtype v1.11.0 // indirect
	github.com/jackc/puddle v1.2.1 // indirect
	github.com/maxsupermanhd/go-mc-ms-auth v0.0.0-20220223195356-5256511fc797
	github.com/pkg/errors v0.9.1 // indirect
	github.com/tklauser/go-sysconf v0.3.10 // indirect
	github.com/tklauser/numcpus v0.5.0 // indirect
	golang.org/x/crypto v0.0.0-20220517005047-85d78b3ac167 // indirect
	golang.org/x/sys v0.0.0-20220513210249-45d2b4557a2a // indirect
	golang.org/x/text v0.3.7 // indirect
	gopkg.in/natefinch/lumberjack.v2 v2.0.0 // indirect
)
