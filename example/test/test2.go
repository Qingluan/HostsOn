package main

import (
	"flag"
	"log"
	"time"

	"gitee.com/dark.H/Jupyter/http"
	"github.com/Qingluan/HostsOn/hosts"
)

var (
	testStr string
)

func main() {
	flag.StringVar(&testStr, "t", "https://ifconfig.co/ip", "curl this url and , show!")
	hosts.DeployOption()

	sess := http.NewSession()
	if resp, err := sess.Get(testStr); err != nil {
		log.Fatal(err)
	} else {
		log.Println(resp.Text())
	}
	time.Sleep(1 * time.Hour)
}
