package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cavaliergopher/grab/v3"
	"github.com/essentialkaos/zip7"
	"github.com/gorilla/pat"
	"github.com/unrolled/render"
)

var rd *render.Render

func Handler(w http.ResponseWriter, r *http.Request) {
	rd.HTML(w, http.StatusOK, "index", "kokily")
}

func WebScrapping() string {
	wg := sync.WaitGroup{}

	wg.Add(1)

	go func() {
		defer wg.Done()

		i := 4

		stop := time.After(5 * time.Second)

		for {
			select {
			case <-stop:
				fmt.Println("다운로드를 시작합니다")
				return
			case <-time.After(1 * time.Second):
				fmt.Printf("%d\n", i)
			}
			i--
		}
	}()

	wg.Wait()

	doc, err := goquery.NewDocument("http://localhost:3000")

	if err != nil {
		log.Fatal(err)
	}

	var b bytes.Buffer

	doc.Find("a[href]").Each(func(index int, item *goquery.Selection) {
		href, _ := item.Attr("href")

		b.WriteString(href + ",")
	})

	return b.String()
}

func Downloader() {
	data := WebScrapping()

	target := strings.Split(data, ",")
	count := strings.Count(data, ",")

	request := make(chan *grab.Request)
	response := make(chan *grab.Response)

	// Start 4 workers
	client := grab.NewClient()

	// Waiting Group
	wg := sync.WaitGroup{}

	for i := 0; i < 4; i++ {
		wg.Add(1)

		go func() {
			client.DoChannel(request, response)
			wg.Done()
		}()
	}

	go func() {
		for i := 0; i < count; i++ {
			url := target[i]
			req, err := grab.NewRequest("./data", url)

			if err != nil {
				log.Fatal(err)
			}

			fmt.Println(strconv.Itoa(count) + " 파일 중 " + strconv.Itoa(i+1) + "번째 파일 다운로드 중")

			request <- req
		}

		close(request)

		wg.Wait()
		close(response)
	}()

	for resp := range response {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()

	Loop:
		for {
			select {
			case <-t.C:
				fmt.Print("..")
			case <-resp.Done:
				break Loop
			}
		}

		if err := resp.Err(); err != nil {
			log.Fatal(err)
		}
	}

	fmt.Printf("%d 개 파일 다운로드 완료\n", count)
}

func ExtractArchive(archive string) {
	fmt.Printf("%s 압축 해제 중\n", strings.Split(archive, ",")[0])

	out, err := zip7.Extract(zip7.Props{
		File:      archive,
		OutputDir: "./data",
		Delete:    true,
	})

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s \n", out)
}

func main() {
	// Web server start on 3000 port
	rd = render.New(render.Options{
		Directory:  "template",
		Extensions: []string{".html", ".tmpl"},
	})

	mux := pat.New()

	mux.Get("/", Handler)

	go func() {
		log.Fatal(http.ListenAndServe(":3000", mux))
	}()

	// Start Scrapping & Download
	Downloader()

	// Start Extract
	ext := os.Args[1]
	dir := "./data/"

	files, err := ioutil.ReadDir(dir)

	if err != nil {
		log.Fatal(err)
	}

	for _, f := range files {
		if filepath.Ext(f.Name()) == "."+ext {
			ExtractArchive(dir + f.Name())
		}
	}
}
