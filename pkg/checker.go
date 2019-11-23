package pkg

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

//HTTPReg http regex
const HTTPReg = `((http|ftp|https)://)(([a-zA-Z0-9\._-]+\.[a-zA-Z]{2,6})|([0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}\.[0-9]{1,3}))(:[0-9]{1,4})*(/[a-zA-Z0-9\&%_\./-~-]*)?`

type Config struct {
	Repo        string
	Branch      string
	Proxy       string
	AccessUser  string
	AccessToken string
	Output      string
	Verbose     bool
	Timeout     int
	Exclude     string
	Help        bool
}

type DeadLinkChecker struct {
	cfg        *Config
	httpClient *http.Client
	cachedURLs map[string]int
}

func NewChecker(cfg *Config) *DeadLinkChecker {
	d := &DeadLinkChecker{cfg: cfg, httpClient: nil, cachedURLs: make(map[string]int)}
	d.setUpHTTPClient()
	d.prepareOutput()
	return d
}

func (c *DeadLinkChecker) prepareOutput() {
	if c.cfg.Output != "" {
		if err := ioutil.WriteFile(c.cfg.Output, make([]byte, 0), os.FileMode(os.O_CREATE|os.O_WRONLY)); err != nil {
			log.Fatal(err)
		}
	}
}

func (c *DeadLinkChecker) setUpHTTPClient() {
	if c.httpClient != nil {
		return
	}
	trans := &http.Transport{
		DisableKeepAlives: true,
	}
	if c.cfg.Proxy != "" {
		uRL := url.URL{}
		urlProxy, err := uRL.Parse(c.cfg.Proxy)
		if err != nil {
			log.Fatal(err)
		}
		trans.Proxy = http.ProxyURL(urlProxy)
	}
	c.httpClient = &http.Client{
		Timeout:   time.Second * time.Duration(c.cfg.Timeout),
		Transport: trans,
	}
}

func (c *DeadLinkChecker) FetchRepository() (*Trees, error) {
	linkReg, err := regexp.Compile(`github.com/.+/.+$`)
	if err != nil {
		return nil, err
	}
	repoLinks := linkReg.FindAllString(c.cfg.Repo, -1)
	if len(repoLinks) == 0 {
		return nil, fmt.Errorf("invalid Repo url:%s", c.cfg.Repo)
	}
	rs := strings.Split(repoLinks[0], "/")
	log.Println("🚀 start scan ...")

	api := fmt.Sprintf("https://api.github.com/repos/%s/%s/git/trees/%s?recursive=1", rs[1], rs[2], c.cfg.Branch)
	if c.cfg.Verbose {
		log.Println("🐞 start request api: ", api)
	}
	rData, err := c.requestAPI(api)
	if err != nil {
		return nil, fmt.Errorf("failed request api %s: %v", api, err)
	}
	var ret Trees
	err = json.Unmarshal([]byte(rData), &ret)
	if err != nil {
		return nil, err
	}
	if ret.Message != "" {
		return nil, fmt.Errorf("fetch repository failed: %s", ret.Message)
	}
	log.Printf("🔗 %d files total in this Repo...\n", len(ret.Tree))
	return &ret, nil
}

func (c *DeadLinkChecker) FilterFile(trees *Trees, fileSuffix string) []Tree {
	fs := make([]Tree, 0)
	excludes := make([]string, 0)
	if c.cfg.Exclude != "" {
		excludes = strings.Split(c.cfg.Exclude, ",")
	}
NextFile:
	for _, t := range trees.Tree {
		for _, e := range excludes {
			if strings.HasPrefix(t.Path, e) {
				continue NextFile
			}
		}
		if t.Type == "blob" && strings.HasSuffix(t.Path, fileSuffix) {
			fs = append(fs, t)
		}
	}
	return fs
}

func (c *DeadLinkChecker) CheckBrokenLink(files []Tree) {
	filesChecked := 0
	reg, err := regexp.Compile(HTTPReg)
	if err != nil {
		log.Fatal(err)
	}
	for _, t := range files {
		errorsCol := make([]string, 0)
		rData, err := c.requestAPI(t.Url)
		if err != nil {
			log.Printf("failed read %s: %v", t.Path, err)
			continue
		}
		var blob Blobs
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
			log.Printf("🕑 checking [%d] links in %s...\n", len(links), t.Path)
			headAppended := false
			for _, link := range links {
				//wg.Add(1)
				//go func(link string) {
				//defer wg.Done()
				if s, e := c.ping(link); e != nil || s != http.StatusOK {
					log.Printf("❌  [%d] %s, %v\n", s, link, e)
					if !headAppended {
						errorsCol = append(errorsCol, "## "+t.Path)
						headAppended = true
					}
					errorsCol = append(errorsCol, fmt.Sprintf("[%d] %s", s, link))
				} else if c.cfg.Verbose {
					log.Printf("✔ [%d] %s\n", s, link)
				}
				//}(l)
			}
			//wg.Wait()
		}
		filesChecked++
		if len(errorsCol) > 0 {
			data := "\r\n" + strings.Join(errorsCol, "\r\n") + "\r\n"
			err := SaveToFile(c.cfg.Output, data)
			if err != nil {
				log.Fatal(err)
			}
		}
		log.Printf("🔲 [%d/%d] done for %s", filesChecked, len(files), t.Path)
	}
}

func (c *DeadLinkChecker) requestAPI(link string) (string, error) {
	req, err := http.NewRequest(http.MethodGet, link, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Connection", "close")
	if c.cfg.AccessToken != "" && c.cfg.AccessUser != "" {
		req.Header.Add("Authorization", "Basic "+base64.StdEncoding.EncodeToString([]byte(c.cfg.AccessUser+":"+c.cfg.AccessToken)))
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
	resp, err = c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = resp.Body.Close()
	}()
	data, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

func (c *DeadLinkChecker) ping(link string) (int, error) {
	if u, ok := c.cachedURLs[link]; ok {
		if c.cfg.Verbose {
			log.Println("use cached url: ", u, link)
		}
		return u, nil
	}
	req, err := http.NewRequest(http.MethodHead, link, nil)
	if err != nil {
		c.cachedURLs[link] = -2
		return -1, err
	}
	var resp *http.Response
	resp, err = c.httpClient.Do(req)
	if err != nil {
		c.cachedURLs[link] = -2
		return -2, err
	}
	c.cachedURLs[link] = resp.StatusCode
	return resp.StatusCode, nil
}
