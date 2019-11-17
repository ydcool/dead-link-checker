/**
./dead-link-checker.exe -v -b master -l github.com/ydcool/QrModule -au ydcool -ak c75c7ff10223322029b467945bc3bb07faaac0d0 -p http://127.0.0.1:3398 -o brokenlinks.txt
*/

package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"git.inspur.com/yindongchao/dead-link-checker/pkg"
)

//HTTPReg http regex
const HTTPReg = `((http|ftp|https)://)(([a-zA-Z0-9\._-]+\.[a-zA-Z]{2,6})|([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}))(:[0-9]{1,4})*(/[a-zA-Z0-9\&%_\./-~-]*)?`

var (
	help        bool
	repo        string
	branch      string
	proxy       string
	accessUser  string
	accessToken string
	output      string
	verbose     bool
	timeout     int

	cachedURLs map[string]int
)

func init() {
	var helpStr = `
Dead Link Checker
  Code with ❤  by Dominic
Usage:
`
	flag.StringVar(&repo, "l", "", `the link/address of target repo to check , for example 'github.com/docker/docker-ce'`)
	flag.StringVar(&branch, "b", "master", "the branch of repository to check, default master")
	flag.StringVar(&proxy, "p", "", `http/https proxy`)
	flag.StringVar(&accessUser, "au", "", `access username for higher requests rate`)
	flag.StringVar(&accessToken, "ak", "", `access token for higher requests rate`)
	flag.StringVar(&output, "o", "", `write result to file`)
	flag.IntVar(&timeout, "timeout", 10, "request timeout in seconds, default 10s")
	flag.BoolVar(&verbose, "v", false, "print all checked links, default false to show broken only")
	flag.BoolVar(&help, "h", false, "help")
	flag.Usage = func() {
		fmt.Print(helpStr)
		flag.PrintDefaults()
	}
}

func main() {
	var (
		err          error
		filesChecked int
	)
	flag.Parse()
	if help || len(os.Args) == 1 {
		flag.Usage()
		return
	}
	if output != "" {
		if err = ioutil.WriteFile(output, make([]byte, 0), os.FileMode(os.O_CREATE|os.O_WRONLY)); err != nil {
			log.Fatal(err)
		}
	}
	linkReg, err := regexp.Compile(`github.com/.+/.+$`)
	if err != nil {
		log.Fatal(err)
	}
	repoLinks := linkReg.FindAllString(repo, -1)
	if len(repoLinks) == 0 {
		log.Fatal("invalid repo url")
	}
	rs := strings.Split(repoLinks[0], "/")
	log.Println("🚀 start scan ...")

	api := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", rs[1], rs[2], branch)
	if verbose {
		log.Println("🐞 start request api: ", api)
	}
	rData, err := doRequest(api)
	if err != nil {
		log.Fatalf("failed request api %s: %v", api, err)
	}
	var resultData pkg.Trees
	err = json.Unmarshal([]byte(rData), &resultData)
	if err != nil {
		log.Fatal(err)
	}
	if resultData.Message != "" {
		log.Print("❌  ", resultData.Message)
		return
	}
	log.Printf("🔗 %d files total in this repo...\n", len(resultData.Tree))

	reg, err := regexp.Compile(HTTPReg)
	if err != nil {
		log.Fatal(err)
	}

	allMarkdown := make([]pkg.Tree, 0)
	for _, t := range resultData.Tree {
		if t.Type == "blob" && strings.HasSuffix(t.Path, ".md") {
			allMarkdown = append(allMarkdown, t)
		}
	}
	log.Printf("🔰 %d md total in this repo...\n", len(allMarkdown))

	cachedURLs = make(map[string]int)

	for _, t := range allMarkdown {
		errorsCol := make([]string, 0)
		rData, err := doRequest(t.Url)
		if err != nil {
			log.Printf("failed read %s: %v", t.Path, err)
			continue
		}
		var blob pkg.Blobs
		err = json.Unmarshal([]byte(rData), &blob)
		if err != nil {
			log.Printf("faild parse blob %s: %v", t.Path, err)
			continue
		}
		if blob.Encoding == "base64" {
			log.Printf("🕑 start scan %s...\n", t.Path)
			//wg := sync.WaitGroup{}
			contentBytes, err := base64.StdEncoding.DecodeString(blob.Content)
			if err != nil {
				log.Printf("failed to decode content of %s : %v", t.Path, err)
				continue
			}
			links := reg.FindAllString(string(contentBytes), -1)
			headAppended := false
			for _, link := range links {
				//wg.Add(1)
				//go func(link string) {
				//defer wg.Done()
				if s, e := doPing(link); e != nil || s != http.StatusOK {
					log.Printf("❌  [%d] %s, %v\n", s, link, e)
					if !headAppended {
						errorsCol = append(errorsCol, "## "+t.Path)
						headAppended = true
					}
					errorsCol = append(errorsCol, fmt.Sprintf("[%d] %s", s, link))
				} else if verbose {
					log.Printf("✔ [%d] %s\n", s, link)
				}
				//}(l)
			}
			//wg.Wait()
		}
		filesChecked++
		if len(errorsCol) > 0 {
			err := saveToFile(output, errorsCol)
			if err != nil {
				log.Fatal(err)
			}
		}
		log.Printf("🔲 [%d/%d] done for %s", filesChecked, len(allMarkdown), t.Path)
	}
}

func saveToFile(file string, data []string) error {
	if file != "" {
		f, err := os.OpenFile(file, os.O_WRONLY|os.O_APPEND, os.FileMode(0644))
		if err != nil {
			return err
		}
		n, err := f.Write([]byte("\r\n" + strings.Join(data, "\r\n") + "\r\n"))
		if err == nil && n < len(data) {
			err = io.ErrShortWrite
		}
		if err1 := f.Close(); err == nil {
			err = err1
		}
		return err
	}
	return nil
}

func getHTTPClient(p string) *http.Client {
	if p != "" {
		uRL := url.URL{}
		urlProxy, _ := uRL.Parse(p)
		c := http.Client{
			Timeout: time.Second * time.Duration(timeout),
			Transport: &http.Transport{
				Proxy: http.ProxyURL(urlProxy),
			},
		}
		return &c
	}
	return &http.Client{
		Timeout: time.Second * time.Duration(timeout),
	}
}

func doRequest(link string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Connection", "close")
	if accessToken != "" && accessUser != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(accessUser+":"+accessToken)))
	}
	var resp *http.Response
	defer func() {
		if resp != nil {
			err = resp.Body.Close()
			if err != nil {
				log.Print(err)
			}
		}
	}()
	resp, err = getHTTPClient(proxy).Do(req)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func doPing(link string) (int, error) {
	if c, ok := cachedURLs[link]; ok {
		if verbose {
			log.Println("use cached url: ", c, link)
		}
		return c, nil
	}
	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		cachedURLs[link] = -2
		return -1, err
	}
	var resp *http.Response
	resp, err = getHTTPClient(proxy).Do(req)
	if err != nil {
		cachedURLs[link] = -2
		return -2, err
	}
	cachedURLs[link] = resp.StatusCode
	return resp.StatusCode, nil
}
