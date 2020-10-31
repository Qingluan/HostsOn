package main

import (
	"flag"
	"log"

	"github.com/Qingluan/HostsOn/hosts"
	"golang.org/x/crypto/ssh"
)

var (
	file  string
	host  string
	shell string
)

func main() {
	flag.StringVar(&file, "f", "", "upload file")
	flag.StringVar(&host, "host", "", "upload file")
	flag.StringVar(&shell, "shell", "", "host file")
	flag.Parse()
	if file != "" && host != "" {
		controller := hosts.NewController()
		if err := controller.Uploads(file, host, true, func(name string, cli *ssh.Client) string {
			return controller.RunByClient(cli, "ls /tmp/")
		}); err != nil {
			log.Fatal(err)
		}
	}

}
