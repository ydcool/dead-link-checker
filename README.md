# Github Dead Link Checker

## 使用说明

```
$ ./dead-link-checker.exe

Dead Link Checker
  Code with ❤  by Dominic
Usage:
  -ak string
        access token for higher requests rate
  -au string
        access username for higher requests rate
  -b string
        the branch of repository to check, default master (default "master")
  -exclude string
        exclude directories to skip scan, like 'vendor'. split with ','
  -h    help
  -l string
        the link/address of target repo to check , for example 'github.com/docker/docker-ce'
  -o string
        write result to file
  -p string
        http/https proxy
  -timeout int
        request timeout in seconds, default 10s (default 10)
  -v    print all checked links, default false to show broken only
```

示例：

```
$ ./dead-link-checker.exe -l github.com/ydcool/QrModule -b master -exclude vendor,test -au ydcool -ak c75c7ff10223322029b467945bc3bb07faaac0d0 -o errors.txt
2019/11/01 00:32:14 🚀 start scan ...
2019/11/01 00:32:14 🔗 136 files total in this repo...
2019/11/01 00:32:14 🔰 1 md total in this repo...
2019/11/01 00:32:15 🕑 start scan README.md...
2019/11/01 00:32:16 ❌  https://api.bintray.com/packages/ydcool/maven/QrModule/images/download.svg
2019/11/01 00:32:17 ❌  https://travis-ci.org/Ydcool/QrModule.svg?branch=master
2019/11/01 00:32:36 ❌  http://developer.android.com/reference/android/graphics/PorterDuff.Mode.html
2019/11/01 00:32:36 🏁 All done! broken links in total: 3
2019/11/01 00:32:36 💌 All broken links saved to errors.txt
```