package main

// TODO build index, searching by tags, author and mb even text
// TODO testing nix package
// Goal: low memory consumption
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

type contentIndex struct {
	positions  []int
	titles     map[int]string
	categories map[string][]int
	authors    map[string][]int
}

func main() {
	rand.Seed(time.Now().UnixNano())

	if len(os.Args) < 2 || os.Args[1] == "" {
		fmt.Println("usage: literotigo database.jsonl")
		return
	}

	log.Printf("Starting literotigo. Opening %s", os.Args[1])

	file, err := os.Open(os.Args[1])
	if err != nil {
		log.Fatalf("error opening database: %v", err)
	}

	mmap, err := mmap.Map(file, mmap.RDONLY, 0)
	if err != nil {
		log.Fatalf("error mmapping database: %v", err)
	}
	defer mmap.Unmap()

	idx := buildIndex(mmap)

	tmpl := template.Must(template.New("").Funcs(funcs).ParseFS(views, "*"))

	s := service{
		tmpl: tmpl,
		data: mmap,
		idx:  idx,
	}

	http.HandleFunc("/", s.Index)
	http.HandleFunc("/view", s.View)

	runtime.GC()

	log.Println("Listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func buildIndex(file mmap.MMap) contentIndex {
	var workers = runtime.GOMAXPROCS(0)

	type chunkInfo struct {
		start int
		end   int
	}

	log.Printf("Started indexing using %d workers", workers)
	startDate := time.Now()

	chunkSize := len(file) / workers
	chunks := make([]chunkInfo, workers)

	// Distribute work between N workers
	for i := range chunks {
		start := chunkSize * i

		// Adjust borders
		if i > 0 {
			start = start + bytes.IndexByte(file[start:], byte('\n'))
			chunks[i-1].end = start + 1
		}

		chunks[i].start = start
	}

	chunks[workers-1].end = len(file)

	results := make([]chan contentIndex, workers)

	for n, c := range chunks {
		results[n] = make(chan contentIndex)
		go indexWorker(n, file[c.start:c.end], results[n])
	}

	idx := contentIndex{
		positions:  []int{},
		titles:     map[int]string{},
		authors:    map[string][]int{},
		categories: map[string][]int{},
	}

	// Merge results
	// Kinda ugly
	for i, c := range results {
		c := <-c

		start := len(idx.positions) - 1
		if i == 0 {
			start = 0
		}

		for _, pos := range c.positions {
			idx.positions = append(idx.positions, chunks[i].start+pos)
		}

		for id, title := range c.titles {
			idx.titles[start+id] = title
		}

		for a, ids := range c.authors {
			for _, id := range ids {
				idx.authors[a] = append(idx.authors[a], start+id)
			}
		}

		for c, ids := range c.categories {
			for _, id := range ids {
				idx.categories[c] = append(idx.categories[c], start+id)
			}
		}
	}

	log.Printf(
		"Finished indexing %d records in %f seconds",
		len(idx.positions),
		time.Now().Sub(startDate).Seconds(),
	)

	return idx
}

func indexWorker(n int, file []byte, resultChan chan<- contentIndex) {
	idx := contentIndex{
		positions:  []int{},
		titles:     map[int]string{},
		authors:    map[string][]int{},
		categories: map[string][]int{},
	}

	r := bytes.NewBuffer(file)

	var pos int

	for i := 0; ; i++ {
		l, err := r.ReadString(byte('\n'))
		if err == io.EOF {
			break
		}

		if len(l) == 1 {
			continue
		}

		var res struct {
			Meta meta `json:"meta"`
		}

		if err = json.Unmarshal([]byte(l), &res); err != nil {
			log.Fatalf("error parsing json: %v", err)
		}

		idx.titles[i] = res.Meta.Title
		idx.authors[res.Meta.Author] = append(idx.authors[res.Meta.Author], i)
		idx.categories[res.Meta.Category] = append(idx.categories[res.Meta.Category], i)

		idx.positions = append(idx.positions, pos)
		pos += len(l)
	}

	resultChan <- idx
}

type service struct {
	tmpl *template.Template
	data mmap.MMap
	idx  contentIndex
}

func (s *service) Index(w http.ResponseWriter, r *http.Request) {
	data := make(map[string]interface{})

	type entry struct {
		ID    int
		Title string
	}

	// 20 random
	{
		list := make([]entry, 20)

		for i := range list {
			id := rand.Intn(len(s.idx.positions))
			title := s.idx.titles[id]

			list[i] = entry{id, title}
		}

		data["title"] = "20 Random"
		data["list"] = list
	}

	if tag := r.URL.Query().Get("tag"); tag != "" {
		list := make([]entry, 0)

		for _, id := range s.idx.categories[tag] {
			list = append(list, entry{id, s.idx.titles[id]})
		}

		data["title"] = tag
		data["list"] = list
	}

	if author := r.URL.Query().Get("author"); author != "" {
		list := make([]entry, 0)

		for _, id := range s.idx.authors[author] {
			list = append(list, entry{id, s.idx.titles[id]})
		}

		data["title"] = "From " + author
		data["list"] = list
	}

	data["tags"] = s.idx.categories // all tags

	if err := s.tmpl.ExecuteTemplate(w, "index.html", data); err != nil {
		s.renderError(w, err, "error executing template")
		return
	}
}

func (s *service) View(w http.ResponseWriter, r *http.Request) {
	n := r.URL.Query().Get("id")
	if n == "" {
		s.renderError(w, nil, "missing id param")
		return
	}

	id, err := strconv.ParseUint(n, 10, 64)
	if err != nil {
		s.renderError(w, err, "error while parsing query")
		return
	}

	start := s.idx.positions[id]

	end := len(s.data)
	if int(id)+1 != len(s.idx.positions) {
		end = s.idx.positions[id+1]
	}

	var res entry
	if err := json.Unmarshal(s.data[start:end], &res); err != nil {
		s.renderError(w, err, "error while parsing json")
		return
	}
	res.Meta.ID = int(id)

	if err := s.tmpl.ExecuteTemplate(w, "view.html", res); err != nil {
		s.renderError(w, err, "error executing template")
	}
}

func (s *service) renderError(w http.ResponseWriter, err error, msg string) {
	e := fmt.Sprintf("%s: %v", msg, err)
	log.Printf(e)
	http.Error(w, e, 500)
}
