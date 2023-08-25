package main

import (
	"bytes"
	"embed"
	"encoding/json"
	"flag"
	"html/template"
	"log"
	"math/rand"
	"net/http"
	"os"
)

//go:embed templates
var indexHTML embed.FS

//go:embed static
var staticFiles embed.FS

var lastEffect = 1

func randEffect() int {
	effect := rand.Intn(5) + 1
	if effect == lastEffect || effect == 1 {
		return randEffect()
	}
	lastEffect = effect
	return effect
}

func main() {
	port := flag.String("port", os.Getenv("PORT"), "port to serve on")
	wledHost := flag.String("wled-host", os.Getenv("WLED_HOST"), "WLED host")
	effectTimeout := flag.Int("effect-timeout", 60*5, "effect timeout in seconds")
	flag.Parse()
	if *port == "" {
		*port = "3000"
	}
	if *wledHost == "" {
		log.Fatal("WLED_HOST is required")
	}

	log.Println("Hello Matelight Public Control!")
	log.Printf("- Talking to WLED at %s\n", *wledHost)
	log.Printf("- Switching back to default effect after %d seconds\n", *effectTimeout)

	// Note the call to ParseFS instead of Parse
	t, err := template.ParseFS(indexHTML, "templates/index.html.tmpl")
	if err != nil {
		log.Fatal(err)
	}

	// http.FS can be used to create a http Filesystem
	var staticFS = http.FS(staticFiles)
	fs := http.FileServer(staticFS)

	// Serve static files
	http.Handle("/static/", fs)

	// Handle only the root path
	http.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		var path = req.URL.Path
		if path != "/" {
			w.WriteHeader(http.StatusNotFound)
			return
		}

		w.Header().Add("Content-Type", "text/html")

		// respond with the output of template execution
		t.Execute(w, struct {
			Title    string
			Response string
		}{Title: "hello", Response: path})
	})
	http.HandleFunc("/random", func(w http.ResponseWriter, req *http.Request) {
		effect := randEffect()

		// use a single item playlist to switch back to the default effect after defined timeout
		obj := map[string]interface{}{
			"playlist": map[string]interface{}{
				"ps":         []int{effect},
				"dur":        []int{*effectTimeout * 10}, // in tenths of a second
				"transition": 0,
				"repeat":     1,
				"end":        1, // default effect: 1
			},
		}

		json, err := json.Marshal(obj)
		if err != nil {
			return
		}

		// log the request
		log.Println(string(json))

		// post to wled
		client := &http.Client{}
		response, err := client.Post(*wledHost+"/json/state", "application/json", bytes.NewBuffer(json))
		if err != nil {
			log.Println(err)

			w.Header().Add("Location", "/")
			w.WriteHeader(http.StatusFound)
			return
		}
		defer response.Body.Close()

		// redirect to home
		w.Header().Add("Location", "/")
		w.WriteHeader(http.StatusFound)
	})

	log.Println("- Listening on port", *port)
	err = http.ListenAndServe(":"+*port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
