/**
./dead-link-checker.exe -v -b master -l github.com/ydcool/QrModule -au ydcool -ak c75c7ff10223322029b467945bc3bb07faaac0d0 -p http://127.0.0.1:3398 -o brokenlinks.txt
*/

package main

import (
	"flag"
	"fmt"
	"git.inspur.com/yindongchao/dead-link-checker/pkg"
	"log"
	"os"
)

var cfg *pkg.Config

func init() {
	cfg = &pkg.Config{}
	var helpStr = `
Dead Link Checker
  Code with ❤  by Dominic
Usage:
`
	flag.StringVar(&cfg.Repo, "l", "", `the link/address of target repo to check , for example 'github.com/docker/docker-ce'`)
	flag.StringVar(&cfg.Branch, "b", "master", "the branch of repository to check, default master")
	flag.StringVar(&cfg.Exclude, "exclude", "", "exclude directories to skip scan, like 'vendor'. split with ','")
	flag.StringVar(&cfg.Proxy, "p", "", `http/https proxy`)
	flag.StringVar(&cfg.AccessUser, "au", "", `access username for higher requests rate`)
	flag.StringVar(&cfg.AccessToken, "ak", "", `access token for higher requests rate`)
	flag.StringVar(&cfg.Output, "o", "", `write result to file`)
	flag.IntVar(&cfg.Timeout, "timeout", 10, "request timeout in seconds, default 10s")
	flag.BoolVar(&cfg.Verbose, "v", false, "print all checked links, default false to show broken only")
	flag.BoolVar(&cfg.Help, "h", false, "help")
	flag.Usage = func() {
		fmt.Print(helpStr)
		flag.PrintDefaults()
	}
}

func main() {
	flag.Parse()
	if cfg.Help || len(os.Args) == 1 {
		flag.Usage()
		return
	}

	checker := pkg.NewChecker(cfg)
	allFiles, err := checker.FetchRepository()
	if err != nil {
		log.Fatal(err)
	}

	allMd := checker.FilterFile(allFiles, ".md")
	log.Printf("🔰 %d md total in this repo...\n", len(allMd))

	checker.CheckBrokenLink(allMd)
}
