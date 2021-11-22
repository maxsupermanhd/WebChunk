package main

import (
	"context"
	"log"
	"net/http"

	"github.com/imdario/mergo"
)

const (
	keyUserUsername       = "User.Username"
	keyUserAuthorized     = "UserAuthorized"
	valUserAuthorizedTrue = "True"
)

func sessionAppendUser(r *http.Request, a *map[string]interface{}) *map[string]interface{} {
	if !checkUserAuthorized(r) {
		return nil
	}
	var sessid int
	sessuname := sessionGetUsername(r)

	if sessuname != "" {
		log.Printf("User: [%s]", sessuname)
		derr := dbpool.QueryRow(context.Background(), `SELECT id FROM users WHERE username = $1`, sessuname).Scan(&sessid)
		if derr != nil {
			log.Println("sessionAppendUser: " + derr.Error())
			return nil
		}
	}
	var usermap map[string]interface{}
	usermap = map[string]interface{}{
		"Username": sessuname,
		"Id":       sessid,
	}
	mergo.Merge(a, map[string]interface{}{
		"UserAuthorized": "True",
		"User":           usermap,
	})
	return a
}

func sessionGetUsername(r *http.Request) string {
	return sessionManager.GetString(r.Context(), keyUserUsername)
}

func checkUserAuthorized(r *http.Request) bool {
	return !(!sessionManager.Exists(r.Context(), keyUserUsername) || sessionManager.Get(r.Context(), keyUserAuthorized) != valUserAuthorizedTrue)
}
