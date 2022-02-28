package main

import (
	"context"
	"net/http"
	"regexp"
	"strconv"

	"github.com/georgysavva/scany/pgxscan"
	"github.com/gorilla/mux"
	"github.com/jackc/pgx/v4"
)

type DimStruct struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Alias    string `json:"alias"`
	ServerID int    `json:"server"`
}

var (
	dimNameRegexp  = regexp.MustCompile(`[\-a-zA-Z0-9.]+`)
	dimAliasRegexp = dimNameRegexp
)

func listDimensionsByServerName(server string) ([]DimStruct, error) {
	var dims []DimStruct
	derr := pgxscan.Select(context.Background(), dbpool, &dims,
		`SELECT dimensions.id, dimensions.name, dimensions.alias, server FROM dimensions JOIN SERVERS ON dimensions.server = servers.id WHERE servers.name = $1`, server)
	return dims, derr
}

func listDimensionsByServerID(sid int) ([]DimStruct, error) {
	var dims []DimStruct
	derr := pgxscan.Select(context.Background(), dbpool, &dims,
		`SELECT id, name, alias, server FROM dimensions WHERE server = $1`, sid)
	return dims, derr
}

func listDimensions() ([]DimStruct, error) {
	var dims []DimStruct
	derr := pgxscan.Select(context.Background(), dbpool, &dims,
		`SELECT id, name, alias, server FROM dimensions`)
	return dims, derr
}

func getDimensionByID(did int) (DimStruct, error) {
	var dim DimStruct
	derr := pgxscan.Select(context.Background(), dbpool, &dim,
		`SELECT id, name, alias, server FROM dimensions WHERE id = $1`, did)
	return dim, derr
}

func getDimensionByNames(server, dimension string) (DimStruct, error) {
	var dim DimStruct
	derr := pgxscan.Select(context.Background(), dbpool, &dim,
		`SELECT id, name, alias, server FROM dimensions`+
			`JOIN SERVERS ON dimensions.server = servers.id`+
			`WHERE dimensions.name = $1 AND servers.name = $2`, dimension, server)
	return dim, derr
}

func addDimension(server int, name, alias string) (DimStruct, error) {
	var dim DimStruct
	derr := pgxscan.Select(context.Background(), dbpool, &dim.ID,
		`INSERT INTO dimensions (server, name, alias) VALUES ($1, $2, $3) RETURNING id`, server, name, alias)
	dim.Alias = alias
	dim.Name = name
	dim.ServerID = server
	return dim, derr
}

func dimensionHandler(w http.ResponseWriter, r *http.Request) {
	params := mux.Vars(r)
	sname := params["server"]
	dimname := params["dim"]
	server, derr := getServerByName(sname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			plainmsg(w, r, plainmsgColorRed, "Server not found")
		} else {
			plainmsg(w, r, plainmsgColorRed, "Database query error: "+derr.Error())
		}
		return
	}
	dim, derr := getDimensionByNames(sname, dimname)
	if derr != nil {
		if derr == pgx.ErrNoRows {
			plainmsg(w, r, plainmsgColorRed, "Dimension not found")
		} else {
			plainmsg(w, r, plainmsgColorRed, "Database query error: "+derr.Error())
		}
		return
	}
	basicLayoutLookupRespond("dim", w, r, map[string]interface{}{"Dim": dim, "Server": server})
}

func apiAddDimension(w http.ResponseWriter, r *http.Request) (int, string) {
	if r.ParseForm() != nil {
		return 400, "Unable to parse form parameters"
	}
	name := r.Form.Get("name")
	if !dimNameRegexp.Match([]byte(name)) {
		return 400, "Invalid dimension name"
	}
	alias := r.Form.Get("alias")
	if !dimAliasRegexp.Match([]byte(alias)) {
		return 400, "Invalid dimension alias"
	}
	serverid, err := strconv.Atoi(r.Form.Get("server"))
	if err != nil {
		return 400, "Invalid dimension server id"
	}
	_, err = getServerByID(serverid)
	if err != nil {
		if err == pgx.ErrNoRows {
			return 404, "Server not found"
		}
	}
	dim, err := addDimension(serverid, name, alias)
	if err != nil {
		return 500, "Failed to add server: " + err.Error()
	}
	setContentTypeJson(w)
	return marshalOrFail(200, dim)
}

func apiListDimensions(w http.ResponseWriter, r *http.Request) (int, string) {
	if r.ParseForm() != nil {
		return 400, "Unable to parse form parameters"
	}
	server := r.Form.Get("server")
	var dims []DimStruct
	var err error
	if server == "" {
		dims, err = listDimensions()
		if err != nil {
			return 500, "Database call failed: " + err.Error()
		}
	} else {
		serverid, err := strconv.Atoi(server)
		if err != nil {
			return 400, "Invalid dimension server id"
		}
		dims, err = listDimensionsByServerID(serverid)
		if err != nil {
			return 500, "Database call failed: " + err.Error()
		}
	}
	setContentTypeJson(w)
	return marshalOrFail(200, dims)
}
