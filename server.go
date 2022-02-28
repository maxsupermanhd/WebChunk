package main

import (
	"context"
	"net/http"
	"regexp"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type ServerStruct struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
	IP   string `json:"ip"`
}

var (
	serverNameRegexp = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
	serverIPRegexp   = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
)

func listServers() ([]ServerStruct, error) {
	var servers []ServerStruct
	derr := pgxscan.Select(context.Background(), dbpool, &servers,
		`SELECT id, name, ip FROM servers`)
	return servers, derr
}

func getServerByID(sid int) (ServerStruct, error) {
	var server ServerStruct
	derr := pgxscan.Select(context.Background(), dbpool, &server,
		`SELECT id, name, ip FROM servers WHERE id = $1`, sid)
	return server, derr
}

func getServerByName(servername string) (*ServerStruct, error) {
	var server []ServerStruct
	derr := pgxscan.Select(context.Background(), dbpool, &server,
		`SELECT * FROM servers WHERE name = $1 LIMIT 1`, servername)
	if len(server) > 0 {
		return &server[0], derr
	} else {
		return nil, derr
	}
}

func addServer(name, ip string) (ServerStruct, error) {
	var server ServerStruct
	server.IP = ip
	server.Name = name
	derr := dbpool.QueryRow(context.Background(),
		`INSERT INTO servers (name, ip) VALUES ($1, $2) RETURNING id`, name, ip).Scan(&server.ID)
	return server, derr
}

func serverHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sname := params["server"]
	server, derr := getServerByName(sname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			plainmsg(w, r, plainmsgColorRed, "Server not found")
		} else {
			plainmsg(w, r, plainmsgColorRed, "Database query error (server): "+derr.Error())
			return
		}
	}
	dims, derr := listDimensionsByServerName(sname)
	if derr != nil {
		plainmsg(w, r, plainmsgColorRed, "Database query error (dimensions): "+derr.Error())
		return
	}
	basicLayoutLookupRespond("server", w, r, map[string]interface{}{"Dims": dims, "Server": server})
}

func apiAddServer(w http.ResponseWriter, r *http.Request) (int, string) {
	if r.ParseMultipartForm(0) != nil {
		return 400, "Unable to parse form parameters"
	}
	name := r.FormValue("name")
	if !serverNameRegexp.Match([]byte(name)) {
		return 400, "Invalid server name"
	}
	ip := r.FormValue("ip")
	if !serverIPRegexp.Match([]byte(ip)) {
		return 400, "Invalid server ip"
	}
	server, err := addServer(name, ip)
	if err != nil {
		return 500, "Failed to add server: " + err.Error()
	}
	setContentTypeJson(w)
	return marshalOrFail(200, server)
}

func apiListServers(w http.ResponseWriter, r *http.Request) (int, string) {
	servers, err := listServers()
	if err != nil {
		return 500, "Database call failed: " + err.Error()
	}
	setContentTypeJson(w)
	return marshalOrFail(200, servers)
}
