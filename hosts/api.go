package hosts

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gitee.com/dark.H/Jupyter/http"
	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

var (
	ifDeamon bool
	logFile  string
)

type Hosts struct {
	args       []string
	IfRemove   bool
	hosts      []string
	remoteFile map[string]string
}

func NewHosts() (hosts *Hosts) {
	hosts = new(Hosts)
	hosts.remoteFile = make(map[string]string)
	hosts.args = os.Args[1:]
	return
}

func (api *Hosts) AddUpload(file string) {
	api.remoteFile[file] = api.ID(file)
}

func (api *Hosts) SetArgs(args []string) {
	api.args = args
}

func GetID(f string) string {
	// absPath, err := filepath.Abs(f)
	// if err != nil {
	// 	log.Fatal("err with parse id:" + f)
	// }
	// U := uuid.NewMD5(uuid.UUID{}, []byte(absPath))
	// return U.String()

	U := uuid.NewMD5(uuid.UUID{}, []byte(f))
	name := filepath.Join(REMOTE_TMP, U.String())
	return name
}

func (api *Hosts) ID(f string) string {
	absPath, err := filepath.Abs(f)
	if err != nil {
		log.Fatal("err with parse id:" + f)
	}
	U := uuid.NewMD5(uuid.UUID{}, []byte(absPath))
	return U.String()
}

func (api *Hosts) SelectHostsByVultr(key string) {
	sess := http.NewSession()
	if key == "" {
		fmt.Print("Read Vultr API KEY:")
		key, _ = bufio.NewReader(os.Stdin).ReadString('\n')

	}
	sess.SetHeader("API-Key", key)
	if resp, err := sess.Get("https://api.vultr.com/v1/server/list"); err != nil {
		log.Fatal(err)
	} else {
		jdata := resp.Json()
		for _, v := range jdata {
			dd := v.(map[string]interface{})
			fmt.Println(dd["main_ip"].(string) + "@root:" + dd["default_password"].(string) + " ##location: " + dd["location"].(string))
		}
	}
}

func (api *Hosts) Self() string {
	return api.args[0]
}

// func (api *Hosts) RemotePath(otherDir ...string) string {
// 	if otherDir == nil {
// 		return filepath.Join("/tmp", api.ID())
// 	}
// 	return filepath.Join(otherDir[0], api.ID())
// }

// if return id > 0 is running
func (api *Hosts) GetRemoteUID(name string, cli *ssh.Client) int {

	res := RunByClient(cli, fmt.Sprintf("ps aux | grep %s | egrep -v \"(grep|egrep)\" | awk '{print $2}' |xargs ", api.ID(name)))
	if strings.TrimSpace(res) == "" {
		return -1
	}
	e, err := strconv.Atoi(strings.TrimSpace(res))
	if err != nil {
		return -1
	}
	return e
}

func (api *Hosts) SelectByHostsFile(f string) {
	buf, err := ioutil.ReadFile(f)
	if err != nil {
		return
	}
	// go func() {
	// 	for {
	// b := <-bar.bars
	// if b != nil{
	// b.
	// }
	// 	}
	// }()
	for _, l := range strings.Split(string(buf), "\n") {
		if strings.TrimSpace(l) != "" {
			api.hosts = append(api.hosts, strings.TrimSpace(l))
		}
	}
}

func (api *Hosts) Upload() {
	controller := NewController()
	for f := range api.remoteFile {
		controller.UploadsByHosts(api.hosts, f, api.IfRemove, func(newname string, cli *ssh.Client) string {
			fmt.Println(cli.RemoteAddr(), newname)
			return "[Upload ok]"
		})
	}

}

func (api *Hosts) RunShell(f string) {
	controller := NewController()
	controller.OnlyRun(api.hosts, func(cli *ssh.Client) string {
		res := RunByClient(cli, f)
		for k, v := range api.remoteFile {
			if strings.Contains(f, k) {
				f = strings.ReplaceAll(f, k, filepath.Join(REMOTE_TMP, v))
			}
		}
		return cli.RemoteAddr().String() + "\n" + res
		// return res
	})
}

// func (api *Hosts) Run() int {
// 	controller := NewController()
// 	controller.UploadsByHosts(api.hosts, api.Self(), api.IfRemove, func(newname string, cli *ssh.Client) string {
// 		res := RunByClient(cli, strings.Join(append([]string{newname}, api.args[1:]...), " "))
// 		fmt.Println(cli.RemoteAddr(), res)
// 		return res
// 	})
// 	return -1
// }

func Daemon(args []string, LOG_FILE string) {
	// defer os.Remove(LOG_FILE)
	if os.Getppid() != 1 {
		createLogFile := func(fileName string) (fd *os.File, err error) {
			dir := path.Dir(fileName)
			if _, err = os.Stat(dir); err != nil && os.IsNotExist(err) {
				if err = os.MkdirAll(dir, 0755); err != nil {
					log.Println(err)
					return
				}
			}

			if fd, err = os.Create(fileName); err != nil {
				log.Println(err)
				return
			}
			return
		}
		if LOG_FILE != "" {
			logFd, err := createLogFile(LOG_FILE)
			if err != nil {
				log.Println(err)
				return
			}
			defer logFd.Close()

			cmdName := args[0]
			newProc, err := os.StartProcess(cmdName, args, &os.ProcAttr{
				Files: []*os.File{logFd, logFd, logFd},
			})
			if err != nil {
				log.Fatal("daemon error:", err)
				return
			}
			log.Printf("Start-Deamon: run in daemon success, pid: %v\nlog : %s", newProc.Pid, LOG_FILE)

		} else {
			cmdName := args[0]
			newProc, err := os.StartProcess(cmdName, args, &os.ProcAttr{
				Files: []*os.File{nil, nil, nil},
			})

			if err != nil {
				log.Fatal("daemon error:", err)
				return
			}
			log.Printf("Start-Deamon: run in daemon success, pid: %v\n", newProc.Pid)

		}

		return
	}
}

func AddDaemonOption() {
	flag.StringVar(&logFile, "log", "daemon.log", "set daemon log file")
	flag.BoolVar(&ifDeamon, "D", false, "true to set daemon")
	flag.Parse()
	if ifDeamon {
		newArgs := []string{}
		for _, o := range os.Args {
			if o == "-D" {
				continue
			} else {
				newArgs = append(newArgs, o)
			}
		}
		Daemon(newArgs, logFile)
		time.Sleep(2 * time.Second)
		// return
		os.Exit(0)
	}
}

func addDeployOp() (action int, prefix string, hs []string, up map[string]int) {
	var hostsFile string
	// flag.StringVar(&hostsFile, "dephosts", "", "set hosts file to deploy")
	// flag.Parse()
	// flag.StringVar(&hostsFile, "hosts","","set hosts file to deploy")
	start := false
	newargs := []string{}
	ifhelp := false
	collectionone := false
	collectionsplit := false
	readPrefix := false
	showProgress := false
	uploads := make(map[string]int)
	for _, a := range os.Args {

		if a == "--By" {
			start = true
			continue
		} else if a == "--dep" {
			action = 1
			continue
		} else if a == "--status" {
			action = 2
			continue
		} else if a == "--kill" {
			action = 3
			continue
		} else if a == "-h" {
			ifhelp = true
		} else if a == "--shell" {
			action = 4
			continue
		} else if a == "--up" {
			action = 5
			collectionone = true
			continue
		} else if a == "--ups" {
			action = 5
			collectionsplit = true
			continue
		} else if a == "--ps" {
			showProgress = true
			continue
		} else if a == "--prefix" {
			readPrefix = true
			continue
		} else {
			if start {
				hostsFile = a
				start = false
				continue
			} else if collectionone && action == 5 {
				uploads[a] = 0
				continue
			} else if collectionsplit && action == 5 {
				uploads[a] = 1
				continue
			} else if readPrefix {
				prefixBuf, err := ioutil.ReadFile(a)
				if err != nil {
					log.Fatal("--prefix read file error:", err)
				}
				prefix = string(prefixBuf)
				readPrefix = false
				continue
			} else if showProgress {
				uploads[a] = 3
				continue
			}
		}

		newargs = append(newargs, a)
	}
	if ifhelp {
		log.Println(`
Add deploy options:
	"--dep" # will -dep this file like: "` + strings.Join(os.Args, " ") + "-D" + `" in remote` + `
	"--status" # will show ` + os.Args[0] + ` in remote's status
	"--ps" [name] # will show  in remote's status
	"--By" # set hosts file , if --By v # will type vultr key to generate hosts file content 
	"--prefix" # will read a file as prefix , for --upload file !!! --upload must in last args
	"--up" #  uplaod file follow this options, then upload to remotes' ` + REMOTE_TMP + `/$base_name(file) , 
	"--ups" # can mark witch file to uplaod before , if set --prefix , will split file by lines  ${each_lines} + ${prefix} --> a tmp file , then uplaod this tmp file , 
	"--kill" # will kill in remote`)
		flag.Parse()
		os.Exit(0)
	}
	if hostsFile == "v" {
		sess := http.NewSession()
		fmt.Print("Read Vultr API KEY:")
		key, _ := bufio.NewReader(os.Stdin).ReadString('\n')
		sess.SetHeader("API-Key", strings.TrimSpace(key))
		if resp, err := sess.Get("https://api.vultr.com/v1/server/list"); err != nil {
			log.Fatal(err)
		} else {
			jdata := resp.Json()
			for _, v := range jdata {
				dd := v.(map[string]interface{})
				fmt.Println(dd["main_ip"].(string) + "@root:" + dd["default_password"].(string) + " ##location: " + dd["location"].(string))
			}
			os.Exit(0)
		}

	} else if hostsFile != "" {
		hs = ReadHostsFile(hostsFile)

	}
	os.Args = newargs
	// hs = hosts
	up = uploads
	return
}

func DeployOption() {

	action, prefix, hosts, uploads := addDeployOp()
	if action == 4 && len(hosts) > 0 {
		con := NewHosts()
		con.hosts = hosts
		con.RunShell(strings.Join(os.Args[1:], " "))
		os.Exit(0)
	}
	AddDaemonOption()
	if action == 1 {
		conn := NewController()
		// for _, f := range uploads {
		// 	conn.SetSplitUpload(f, prefix)
		// 	conn.UploadsByHosts(hosts, f, true, nil)
		// }
		// conn.UploadMode = 0
		// conn.UploadFileName = ""
		log.Println("chmod +x " + GetID(os.Args[0]) + ";" + strings.Join(os.Args, " "))
		conn.UploadsByHosts(hosts, os.Args[0], true, func(newname string, cli *ssh.Client) string {
			os.Args[0] = newname
			os.Args = append(os.Args, "-D")
			// log.Println("Run:", "chmod +x "+newname+";"+strings.Join(os.Args, " "))
			res := RunByClient(cli, "chmod +x "+newname+";"+strings.Join(os.Args, " "))
			return res
		})
		os.Exit(0)
	} else if action == 2 {

		if len(uploads) > 0 {
			conn := NewController()
			newname := ""
			for n := range uploads {
				newname = n
				break
			}
			log.Println("status with:", fmt.Sprintf("ps aux | grep \"%s\" | egrep -v \"(grep|egrep)\" | awk '{print $2}' |xargs ", newname))
			conn.OnlyRun(hosts, func(cli *ssh.Client) string {
				newname := GetID(os.Args[0])
				res := RunByClient(cli, fmt.Sprintf("ps aux | grep \"%s\" | egrep -v \"(grep|egrep)\" | awk '{print $2}' |xargs ", newname))
				if strings.TrimSpace(res) == "" {
					// fmt.Println(cli.RemoteAddr(), " not Running")
					return res
				}
				// e, err := strconv.Atoi(strings.TrimSpace(res))
				// if err != nil {
				// 	fmt.Println("\n", cli.RemoteAddr(), " not Running", res)
				// 	return res
				// }
				fmt.Println(cli.RemoteAddr(), "Running in :", strings.TrimSpace(res))
				return res
			})
		} else {
			conn := NewController()
			newname := GetID(os.Args[0])
			log.Println("status with:", fmt.Sprintf("ps aux | grep \"%s\" | egrep -v \"(grep|egrep)\" | awk '{print $2}' |xargs ", newname))
			conn.OnlyRun(hosts, func(cli *ssh.Client) string {
				newname := GetID(os.Args[0])
				res := RunByClient(cli, fmt.Sprintf("ps aux | grep \"%s\" | egrep -v \"(grep|egrep)\" | awk '{print $2}' |xargs ", newname))
				if strings.TrimSpace(res) == "" {
					// fmt.Println(cli.RemoteAddr(), " not Running")
					return res
				}
				// e, err := strconv.Atoi(strings.TrimSpace(res))
				// if err != nil {
				// 	fmt.Println("\n", cli.RemoteAddr(), " not Running", res)
				// 	return res
				// }
				fmt.Println(cli.RemoteAddr(), "Running in :", strings.TrimSpace(res))
				return res
			})
		}

		os.Exit(0)
	} else if action == 3 {
		// AddDaemonOption()
		conn := NewController()

		conn.OnlyRun(hosts, func(cli *ssh.Client) string {
			newname := GetID(os.Args[0])
			res := RunByClient(cli, fmt.Sprintf("ps aux | grep %s | egrep -v \"(grep|egrep)\" | awk '{print $2}' |xargs kill -9", newname))
			return res
		})
		os.Exit(0)
	} else if action == 5 {
		conn := NewController()
		for n, mode := range uploads {
			if mode == 1 {
				conn.SetSplitUpload(n, prefix)
				conn.UploadsByHosts(hosts, n, true, nil)

			} else {
				conn.SetSingleUpload(n)
				conn.UploadsByHosts(hosts, n, true, nil)
			}
			// conn.UploadsByHosts(hosts, f, t, func(newname string))
		}
		os.Exit(0)
	}

}
