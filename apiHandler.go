package main

import (
	"encoding/json"
	"net/http"
)

func apiHandle(f func(http.ResponseWriter, *http.Request) (int, string)) func(http.ResponseWriter, *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		code, content := f(w, r)
		w.Header().Set("Server", "WebChunk webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		w.WriteHeader(code)
		w.Write([]byte(content))
	}
}

func marshalOrFail(code int, content interface{}) (int, string) {
	resp, err := json.Marshal(content)
	if err != nil {
		return 500, "JSON serialization failed: " + err.Error()
	}
	return code, string(resp) + "\n"
}

func setContentTypeJson(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
}
