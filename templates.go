/*
	WebChunk, web server for block game maps
	Copyright (C) 2022 Maxim Zhuchkov

	This program is free software: you can redistribute it and/or modify
	it under the terms of the GNU Affero General Public License as published
	by the Free Software Foundation, either version 3 of the License, or
	(at your option) any later version.

	This program is distributed in the hope that it will be useful,
	but WITHOUT ANY WARRANTY; without even the implied warranty of
	MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
	GNU Affero General Public License for more details.

	You should have received a copy of the GNU Affero General Public License
	along with this program.  If not, see <https://www.gnu.org/licenses/>.

	Contact me via mail: q3.max.2011@yandex.ru or Discord: MaX#6717
*/

package main

import (
	"context"
	"fmt"
	"html/template"
	"log"
	"net/http"
	"reflect"

	"github.com/davecgh/go-spew/spew"
	"github.com/fsnotify/fsnotify"
	"github.com/maxsupermanhd/lac"
)

const (
	plainmsgColorRed = iota
	plainmsgColorGreen
)

var templates *template.Template
var templatesFuncs = template.FuncMap{
	"noescape": func(s string) template.HTML {
		return template.HTML(s)
	},
	"noescapeJS": func(s string) template.JS {
		return template.JS(s)
	},
	"inc": func(i int) int {
		return i + 1
	},
	"avail": func(name string, data interface{}) bool {
		v := reflect.ValueOf(data)
		if v.Kind() == reflect.Ptr {
			v = v.Elem()
		}
		m, ok := data.(map[string]interface{})
		if ok {
			_, ok := m[name]
			return ok
		}
		if v.Kind() != reflect.Struct {
			return false
		}
		return v.FieldByName(name).IsValid()
	},
	"spew": spew.Sdump,
	"add": func(a, b int) int {
		return a + b
	},
	"FormatBytes":   ByteCountIEC,
	"FormatPercent": FormatPercent,
	"getTypeString": func(a any) string {
		return reflect.TypeOf(a).String()
	},
}

func robotsHandler(w http.ResponseWriter, _ *http.Request) {
	fmt.Fprint(w, "User-agent: *\nDisallow: /\n\n\n")
}
func faviconHandler(w http.ResponseWriter, r *http.Request) {
	http.ServeFile(w, r, "./static/favicon.ico")
}

func plainmsg(w http.ResponseWriter, r *http.Request, color int, msg string) {
	templateRespond("plainmsg", w, r, map[string]interface{}{
		"msgred":   color == plainmsgColorRed,
		"msggreen": color == plainmsgColorGreen,
		"msg":      msg})
}

func templateManager(ctx context.Context, cfg *lac.ConfSubtree) {
	log.Println("Loading web templates")
	templatesGlob := cfg.GetDSString("templates/*.gohtml", "templates_glob")
	var err error
	templates, err = template.New("main").Funcs(templatesFuncs).ParseGlob(templatesGlob)
	if err != nil {
		log.Fatal(err)
	}
	if !cfg.GetDSBool(false, "template_reload") {
		return
	}
	log.Println("Starting filesystem watcher for web templates")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("Failed to create watcher: ", err)
	}
	templatesDir := cfg.GetDSString("templates/", "templates_dir")
	err = watcher.Add(templatesDir)
	if err != nil {
		log.Fatal("Failed to add wathcer path: ", err)
	}
	for {
		select {
		case event, ok := <-watcher.Events:
			if !ok {
				log.Println("Layouts watcher failed to read from events channel")
				return
			}
			log.Println("Event:", event)
			if event.Op&fsnotify.Write == fsnotify.Write {
				log.Println("Updating templates")
				nlayouts, err := template.New("main").Funcs(templatesFuncs).ParseGlob(templatesGlob)
				if err != nil {
					log.Println("Error while parsing templates:", err.Error())
				} else {
					templates = nlayouts.Funcs(templatesFuncs)
				}
			}
		case err, ok := <-watcher.Errors:
			if !ok {
				log.Println("Layouts watcher failed to read from error channel")
				return
			}
			log.Println("Layouts watcher error:", err)
		case <-ctx.Done():
			watcher.Close()
			log.Println("Layouts watcher stopped")
			return
		}
	}
}

func templateRespond(page string, w http.ResponseWriter, _ *http.Request, m map[string]interface{}) {
	in := templates.Lookup(page)
	if in != nil {
		m["NavWhere"] = page
		m["WebChunkVersion"] = fmt.Sprintf("%s %s built %s %s", GitTag, CommitHash, BuildTime, GoVersion)
		w.Header().Set("Server", "WebChunk webserver "+CommitHash)
		w.Header().Set("Cache-Control", "no-cache")
		err := in.Execute(w, m)
		if err != nil {
			log.Println(err)
		}
	} else {
		log.Printf("Template %s not found!", page)
		http.Error(w, "", http.StatusNotFound)
	}
}

func basicTemplateResponseHandler(page string) func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		templateRespond(page, w, r, map[string]any{})
	}
}
