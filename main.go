package main

// TODO build index, searching by tags, author and mb even text
// TODO parallel indexing testing nix package
// Goal: low memory consumption. ideally less than < 20 megs
// Goal: Single ~20 loc mmap dependency

import (
	"bytes"
	"embed"
	_ "embed"
	"encoding/json"
	"fmt"
	"html/template"
	"io"
	"log"
	"math/rand"
	"net/http"
	"os"
	"runtime"
	"strconv"
	"time"

	"github.com/edsrzf/mmap-go"

	_ "net/http/pprof"
)

const entriesCount = 473653 // cat source.jsonl | wc -l

//go:embed *.html
var views embed.FS

var funcs = template.FuncMap{
	"add": func(a, b int) int {
		return a + b
	},
}

type meta struct {
	ID       int
	Title    string `json:"title"`
	Author   string `json:"author"`
	Category string `json:"category"`
}

type entry struct {
	Text template.HTML `json:"text"`
	Meta meta          `json:"meta"`
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) < 2 || os.Args[1] == "" {
		fmt.Println("usage: literotigo database.jsonl")
		return
	}

	log.Printf("Starting literotigo. Opening %s", os.Args[1])

	file, err := os.Open(os.Args[1])
	check(err)

	mmap, err := mmap.Map(file, mmap.RDONLY, 0)
	check(err)
	defer mmap.Unmap()

	r := bytes.NewBuffer(mmap)
	n := 0

	indices := make([]int, 0)
	titles := make(map[int]string)
	authors := make(map[string][]int)
	categories := make(map[string][]int)

	{
		log.Println("Started indexing")

		start := time.Now()
		for i := 0; ; i++ {
			l, err := r.ReadString(byte('\n'))
			if err == io.EOF {
				break
			}

			var res struct {
				Meta meta `json:"meta"`
			}

			err = json.Unmarshal([]byte(l), &res)
			check(err)

			if res.Meta.Category == "" {
				fmt.Println("empty")
			}

			titles[i] = res.Meta.Title
			authors[res.Meta.Author] = append(authors[res.Meta.Author], i)
			categories[res.Meta.Category] = append(categories[res.Meta.Category], i)

			indices = append(indices, n)
			n += len(l)
		}

		log.Printf("Finished indexing in %f seconds", time.Now().Sub(start).Seconds())

		runtime.GC()
	}

	tmpl := template.Must(template.New("").Funcs(funcs).ParseFS(views, "*"))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		data := make(map[string]interface{})

		type entry struct {
			ID    int
			Title string
		}

		// 20 random
		{
			list := make([]entry, 20)

			for i := range list {
				id := rand.Intn(len(indices))
				title := titles[id]

				list[i] = entry{id, title}
			}

			data["title"] = "20 Random"
			data["list"] = list
		}

		if tag := r.URL.Query().Get("tag"); tag != "" {
			list := make([]entry, 0)

			for _, id := range categories[tag] {
				list = append(list, entry{id, titles[id]})
			}

			data["title"] = tag
			data["list"] = list
		}

		if author := r.URL.Query().Get("author"); author != "" {
			list := make([]entry, 0)

			for _, id := range authors[author] {
				list = append(list, entry{id, titles[id]})
			}

			data["title"] = "From " + author
			data["list"] = list
		}

		data["tags"] = categories // all tags

		if err := tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
			renderError(w, err, "error executing template")
			return
		}
	})

	http.HandleFunc("/view", func(w http.ResponseWriter, r *http.Request) {
		n := r.URL.Query().Get("id")
		if n == "" {
			renderError(w, nil, "missing id param")
			return
		}

		id, err := strconv.ParseUint(n, 10, 64)
		if err != nil {
			renderError(w, err, "error while parsing query")
			return
		}

		start := indices[id]
		end := indices[id+1]

		var res entry
		if err := json.Unmarshal(mmap[start:end], &res); err != nil {
			renderError(w, err, "error while parsing json")
			return
		}
		res.Meta.ID = int(id)

		if err := tmpl.ExecuteTemplate(w, "view.html", res); err != nil {
			renderError(w, err, "error executing template")
		}
	})

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func renderError(w http.ResponseWriter, err error, s string) {
	e := fmt.Sprintf("%s: %v", s, err)
	log.Printf(e)
	http.Error(w, e, 500)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
