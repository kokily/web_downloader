package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/cavaliergopher/grab/v3"
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

	spinner := []string{"|", "/", "-", "\\"}

	for resp := range response {
		t := time.NewTicker(500 * time.Millisecond)
		defer t.Stop()

	Loop:
		for {
			select {
			case <-t.C:
				for _, s := range spinner {
					fmt.Printf("\r%s", s)
					time.Sleep(100 * time.Millisecond)
				}
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
	fmt.Printf("%s 압축 해제 중\n", archive)

	cmd := exec.Command("7z", "x", archive, "-odata")

	err := cmd.Run()

	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("%s 압축 해제 완료\n", archive)
}

func RemoveCompress() {
	dir := "data"

	d, err := os.Open(dir)

	if err != nil {
		fmt.Println("data 디렉토리 오픈 실패: ", err)
	}

	defer d.Close()

	files, err := d.ReadDir(-1)

	if err != nil {
		fmt.Println("data 폴더 내 파일 읽어오기 실패: ", err)
	}

	for _, file := range files {
		if strings.Contains(file.Name(), "7z") {
			filePath := filepath.Join(dir, file.Name())

			err := os.Remove(filePath)

			if err != nil {
				fmt.Printf("%s 파일 삭제 실패 : %v\n", filePath, err)
			} else {
				fmt.Printf("%s 파일 삭제\n", filePath)
			}
		}
	}

	fmt.Printf("%d 개 파일 삭제 완료\n", len(files))
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
	ext := "001"
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

	// RemoveCompress()
	RemoveCompress()
}
