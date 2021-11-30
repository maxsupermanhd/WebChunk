package main

import (
	"log"
	"net/http"
)

func plainmsg(w http.ResponseWriter, r *http.Request, color int, msg string) {
	basicLayoutLookupRespond("plainmsg", w, r, map[string]interface{}{
		"msgred":   color == 2,
		"msggreen": color == 1,
		"msg":      msg})
}

func basicLayoutLookupRespond(page string, w http.ResponseWriter, r *http.Request, m map[string]interface{}) {
	in := layouts.Lookup(page)
	if in != nil {
		m["NavWhere"] = page
		// sessionAppendUser(r, &m)
		w.Header().Set("Server", "WebChunk webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		err := in.Execute(w, m)
		if err != nil {
			log.Println(err)
		}
	} else {
		http.Error(w, http.StatusText(http.StatusNotFound), http.StatusNotFound)
	}
}
