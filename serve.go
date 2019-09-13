package eephttpd

import (
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
	"strings"

	"github.com/d5/tengo/script"
)

func (f *EepHttpd) ServeHTTP(rw http.ResponseWriter, rq *http.Request) {
	if strings.HasSuffix(rq.URL.Path, ".md") {
		f.HandleMarkdown(rw, rq)
		return
	}
	if strings.HasSuffix(rq.URL.Path, ".tengo") {
		f.HandleScript(rw, rq)
		return
	}
	f.HandleFile(rw, rq)
}

func (f *EepHttpd) checkURL(rq *http.Request) string {
	p := rq.URL.Path
	if rq.URL.Path == "/" {
		p = "/index.html"
	}
	log.Println(p)
	return filepath.Join(f.ServeDir, p)
}

func (f *EepHttpd) HandleScript(rw http.ResponseWriter, rq *http.Request) {
	path := f.checkURL(rq)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		log.Println(err)
		return
	}
	scr := script.New(bytes)
	com, err := scr.Compile()
	if err != nil {
		log.Println(err)
		panic(err)
	}
	if err := com.Run(); err != nil {
		log.Println(err)
		panic(err)
	}
	response := com.Get("response")
	fmt.Fprintf(rw, response.String())
}

func (f *EepHttpd) HandleMarkdown(rw http.ResponseWriter, rq *http.Request) {
	path := f.checkURL(rq)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		return
	}
	f.mark.Render(rw, bytes)
}

func (f *EepHttpd) HandleFile(rw http.ResponseWriter, rq *http.Request) {
	path := f.checkURL(rq)
	bytes, err := ioutil.ReadFile(path)
	if err != nil {
		f.HandleMissing(rw, rq)
	}
	fmt.Fprintf(rw, string(bytes))
}

func (f *EepHttpd) HandleMissing(rw http.ResponseWriter, rq *http.Request) {
	path := f.checkURL(rq)
	fmt.Fprintf(rw, "ERROR %s NOT FOUND", strings.Replace(path, f.ServeDir, "", -1))
}
