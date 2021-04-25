package main

// TODO build index, searching by tags, author and mb even text
// TODO parallel indexing testing navigation nix package
// Goal: low memory consumption. ideally less than < 20 megs
// Goal: Single 20loc mmap dependency

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"html/template"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/edsrzf/mmap-go"
)

//go:embed index.html
var html string

var funcs = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
}

func main() {
	log.Printf("Starting literotigo. Opening %s", os.Args[1])
	file, err := os.Open(os.Args[1])
	check(err)

	mmap, err := mmap.Map(file, mmap.RDONLY, 0)
	check(err)
	defer mmap.Unmap()

	r := bytes.NewBuffer(mmap)
	n := 0

	indices := make([]int64, 0, 473653)

	{
		log.Println("Started indexing")

		start := time.Now()
		for i := 0; ; i++ {
			l, err := r.ReadString(byte('\n'))
			if err == io.EOF {
				break
			}
			check(err)

			indices = append(indices, int64(n))
			n += len(l)
		}

		log.Printf("Finished indexing in %f seconds", time.Now().Sub(start).Seconds())
	}

	tmpl := template.Must(template.New("").Funcs(funcs).Parse(html))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		type entry struct {
			Text template.HTML `json:"text"`
			Meta struct {
				ID       int
				Title    string `json:"title"`
				Author   string `json:"author"`
				Category string `json:"category"`
			} `json:"meta"`
		}

		var id int
		if n := r.URL.Query().Get("n"); n != "" {
			id, err = strconv.Atoi(n)
			if err != nil {
				http.Error(w, "error while parsing ", 500)
				return
			}
		}

		start := indices[id]
		end := indices[id+1]

		var res entry
		if err := json.Unmarshal(mmap[start:end], &res); err != nil {
			http.Error(w, "error while parsing json", 500)
			return
		}
		res.Meta.ID = id

		if err := tmpl.Execute(w, res); err != nil {
			http.Error(w, "error executing template", 500)
			return
		}
	})

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
