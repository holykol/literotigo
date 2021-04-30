package main

import (
	"html/template"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"syscall"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var data = []byte(`{"text": "text0","meta": {"title": "title0","author": "Alice","category": "funny"}}
{"text": "text1","meta": {"title": "title1","author": "Alice","category": "romance"}}
{"text": "text2","meta": {"title": "title2","author": "Bob","category": "romance"}}
{"text": "text3","meta": {"title": "title3","author": "Alice","category": "serious"}}
{"text": "text4","meta": {"title": "title4","author": "Bob","category": "funny"}}
`)

func TestMain(t *testing.T) {
	if testing.Short() {
		t.SkipNow()
	}

	file, err := ioutil.TempFile("/tmp", "literotigo-*.json")
	assert.Nil(t, err)

	defer os.Remove(file.Name())

	_, err = file.Write(data)
	assert.Nil(t, err)

	os.Args = []string{"test", file.Name()}

	go func() {
		main()
	}()

	defer func() {
		e := syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
		assert.NoError(t, e)
	}()

	time.Sleep(time.Millisecond * 100)

	{
		resp, err := http.Get("http://localhost:8080/?tag=funny")
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := ioutil.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "Tag: funny")
		assert.Contains(t, string(body), "title4")
	}

	{
		resp, err := http.Get("http://localhost:8080/view?id=3")
		assert.Nil(t, err)
		defer resp.Body.Close()

		assert.Equal(t, 200, resp.StatusCode)

		body, err := ioutil.ReadAll(resp.Body)
		assert.Nil(t, err)
		assert.Contains(t, string(body), "title3")
		assert.Contains(t, string(body), "text3")
	}

}
func TestIndex(t *testing.T) {
	idx := buildIndex(data, 3)

	assert.Len(t, idx.positions, 5)
	assert.Equal(t, []int{0, 84, 169, 253, 339}, idx.positions)

	assert.Len(t, idx.titles, 5)
	assert.Equal(t, idx.titles[2], "title2")

	assert.Len(t, idx.categories, 3)
	assert.Equal(t, []int{0, 4}, idx.categories["funny"])
	assert.Equal(t, []int{1, 2}, idx.categories["romance"])
	assert.Equal(t, []int{3}, idx.categories["serious"])

	assert.Len(t, idx.authors, 2)
	assert.Equal(t, []int{0, 1, 3}, idx.authors["Alice"])
	assert.Equal(t, []int{2, 4}, idx.authors["Bob"])
}

func TestService(t *testing.T) {
	tmpl := template.Must(template.New("").Funcs(funcs).ParseFS(views, "*"))

	s := service{tmpl, data, buildIndex(data, 1)}

	t.Run("index", func(t *testing.T) {
		body := assert.HTTPBody(s.Index, "GET", "/", nil)
		assert.Contains(t, body, "20 Random")
		assert.Contains(t, body, "Tags")

		body = assert.HTTPBody(s.Index, "GET", "/", url.Values{"tag": []string{"funny"}})
		assert.Contains(t, body, "Tag: funny")
		assert.Contains(t, body, "title0")
		assert.Contains(t, body, "title4")

		body = assert.HTTPBody(s.Index, "GET", "/", url.Values{"author": []string{"Alice"}})
		assert.Contains(t, body, "From Alice")
		assert.Contains(t, body, "title1")
		assert.Contains(t, body, "title3")
	})

	t.Run("view", func(t *testing.T) {
		body := assert.HTTPBody(s.View, "GET", "/view", url.Values{"id": []string{"1"}})
		assert.Contains(t, body, "title1")
		assert.Contains(t, body, "text1")

		body = assert.HTTPBody(s.View, "GET", "/view", url.Values{"id": []string{"4"}})
		assert.Contains(t, body, "title4")
		assert.Contains(t, body, "text4")

		// Test errors
		assert.HTTPError(t, s.View, "GET", "/view", url.Values{"id": []string{"999"}})
		assert.HTTPError(t, s.View, "GET", "/view", url.Values{"id": []string{"-1"}})
		assert.HTTPError(t, s.View, "GET", "/view", nil)
	})
}
