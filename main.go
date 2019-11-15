/**
./dead-link-checker.exe -v -b master -l github.com/ydcool/QrModule -au ydcool -ak c75c7ff10223322029b467945bc3bb07faaac0d0 -p http://127.0.0.1:3398 -o brokenlinks.txt

TODO
 链接缓存，去重
 0
 请求降频
*/

package main

import (
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"git.inspur.com/yindongchao/dead-link-checker/pkg"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const HttpReg = `((http|ftp|https)://)(([a-zA-Z0-9\._-]+\.[a-zA-Z]{2,6})|([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}))(:[0-9]{1,4})*(/[a-zA-Z0-9\&%_\./-~-]*)?`

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
)

func init() {
	var helpStr = `
Dead Link Checker
  Code with ❤  by Dominic
Usage:
`
	flag.StringVar(&repo, "l", "", `the user name of target repo path to check , for example 'github.com/docker/docker-ce'`)
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
	//log.Print("start..")
	////i, e := DoPing("https://dl.k8s.io/v1.10.13/kubernetes-client-darwin-386.tar.gz")
	//i, e := DoPing("https://api.bintray.com/packages/ydcool/maven/QrModule/images/download.svg")
	//log.Print(i, e)
	//return

	var (
		err          error
		errorsCol    = make([]string, 0)
		filesChecked int
	)
	flag.Parse()
	if help || len(os.Args) == 1 {
		flag.Usage()
		return
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
	rData, err := DoRequest(api)
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

	reg, err := regexp.Compile(HttpReg)
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

	for _, t := range allMarkdown {
		rData, err := DoRequest(t.Url)
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
				if s, e := DoPing(link); e != nil || s != http.StatusOK {
					log.Printf("❌  [%d] %s\n", s, link)
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
			log.Printf("🔲 [%.2f%%] done for %s", float32(filesChecked)/float32(len(allMarkdown))*100, t.Path)

		}
		filesChecked++
	}

	if output != "" {
		err = ioutil.WriteFile(output, []byte(strings.Join(errorsCol, "\r\n")), os.FileMode(os.O_CREATE|os.O_WRONLY))
		if err != nil {
			log.Fatal(err)
		}
		log.Print("🏁 All broken links saved to ", output)
	}
}

func getHttpClient(p string) *http.Client {
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

func DoRequest(link string) (string, error) {
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
	resp, err = getHttpClient(proxy).Do(req)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func DoPing(link string) (int, error) {
	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		return -1, err
	}
	var resp *http.Response
	resp, err = getHttpClient(proxy).Do(req)
	if err != nil {
		return -2, err
	}
	return resp.StatusCode, nil
}
