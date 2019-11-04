/**
./dead-link-checker.exe -v -b master -l github.com/ydcool/QrModule -au ydcool -ak c75c7ff10223322029b467945bc3bb07faaac0d0 -p http://127.0.0.1:3398 -o brokenlinks.txt
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
	"sync"
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
		errorsCol    = make([]string, 0)
		totalFiles   int
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
	repoLinks := linkReg.FindAllString(repo, 1)
	if len(repoLinks) == 0 {
		log.Fatal("invalid repo url")
	}
	rs := strings.Split(repoLinks[0], "/")
	log.Println("🚀 start scan ...")

	rData, err := DoRequest(fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", rs[1], rs[2], branch))
	if err != nil {
		log.Fatal(err)
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
	totalFiles = len(resultData.Tree)
	log.Printf("🔗 %d files total in this repo...\n", totalFiles)

	reg, err := regexp.Compile(HttpReg)
	if err != nil {
		log.Fatal(err)
	}

	for _, t := range resultData.Tree {
		if t.Type == "blob" && strings.HasSuffix(t.Path, ".md") {
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
				wg := sync.WaitGroup{}
				contentBytes, err := base64.StdEncoding.DecodeString(blob.Content)
				if err != nil {
					log.Printf("failed to decode content of %s : %v", t.Path, err)
					continue
				}
				links := reg.FindAllString(string(contentBytes), -1)
				headAppended := false
				for _, l := range links {
					wg.Add(1)
					go func(link string) {
						defer wg.Done()
						if !DoPing(link) {
							log.Printf("❌  %s\n", link)
							if !headAppended {
								errorsCol = append(errorsCol, "## "+t.Path)
								headAppended = true
							}
							errorsCol = append(errorsCol, link)
						} else if verbose {
							log.Printf("✔ %s\n", link)
						}
					}(l)
				}
				wg.Wait()
				log.Printf("🔲 [%.2f%%] done for %s", float32(filesChecked)/float32(totalFiles)*100, t.Path)
			}
		}
		filesChecked++
	}
	log.Println("🏁 All done! broken links in total:", len(errorsCol))

	if output != "" {
		err = ioutil.WriteFile(output, []byte(strings.Join(errorsCol, "\r\n")), os.FileMode(os.O_CREATE|os.O_WRONLY))
		if err != nil {
			log.Fatal(err)
		}
		log.Print("💌 All broken links saved to ", output)
	}
}

func getHttpClient(p string) *http.Client {
	if p != "" {
		uRL := url.URL{}
		urlProxy, _ := uRL.Parse(p)
		c := http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				Proxy: http.ProxyURL(urlProxy),
			},
		}
		return &c
	}
	return &http.Client{
		Timeout: 10 * time.Second,
	}
}

func addAccessHeader(au, ak string, req *http.Request) {
	req.Header.Set("Connection", "close")
	if au != "" && ak != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(au+":"+ak)))
	}
}

func DoRequest(link string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	addAccessHeader(accessUser, accessToken, req)
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

func DoPing(link string) bool {
	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		return false
	}
	addAccessHeader(accessUser, accessToken, req)
	var resp *http.Response
	resp, err = getHttpClient(proxy).Do(req)
	if err != nil {
		return false
	}
	return resp.StatusCode == http.StatusOK
}
