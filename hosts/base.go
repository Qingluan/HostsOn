package hosts

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/schollz/progressbar/v3"

	"gitee.com/dark.H/go-remote-repl/datas"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

type Ssh string

type Auth struct {
	Type   string
	Target string
	User   string
	Pwd    string
	Extend string
}

// protcol://ip:port:user:pwd
// pro://ip:port@user:pwd
func ParseStringToBrute(s string) (a *Auth, err error) {
	fs := strings.SplitN(s, "@", 2)
	if len(fs) != 2 {
		var name, pwd string
		if name == "" {
			name = "root"
		}
		fs = append(fs, name+":"+pwd)
	}
	afs := strings.SplitN(fs[1], ":", 2)
	if len(afs) != 2 {
		return nil, errors.New("must |  user:pwd, you are:" + s)
	}
	// protocl := strings.SplitN(fs[0], "://", 2)[0]
	addr := strings.TrimSpace(fs[0])
	a = new(Auth)
	a.Type = "ssh"
	a.Target = addr
	a.User = afs[0]
	a.Pwd = afs[1]
	return
}

func (sshstr Ssh) GetIdRsa() (ssh.Signer, error) {
	home, _ := os.UserHomeDir()
	file := filepath.Join(home, ".ssh", "id_rsa")
	// log.Println("try to read :", file)
	buf, err := ioutil.ReadFile(file)
	if err != nil {
		fmt.Println(err)

		return nil, err
	}
	// authorizedKeysBytes, _ := ioutil.ReadFile(filepath.Join(home, ".ssh", "id_rsa.pub"))
	// pcert, _, _, _, err := ssh.ParseAuthorizedKey(authorizedKeysBytes)

	upkey, err := ssh.ParsePrivateKey(buf)

	if err != nil {
		fmt.Println(err)
		return nil, err
	}
	return upkey, nil

	// usigner, err := ssh.NewSignerFromKey(upkey)
	// if err != nil {
	//      log.Printf("Failed to create new signer, err: %v", err)
	// }
	// log.Printf("signer: %v", usigner)

	// ucertSigner, err := ssh.NewCertSigner(pcert.(*ssh.Certificate), usigner)

	// if err != nil {
	//      log.Printf("Failed to create new signer, err: %v", err)
	// }

	// return ucertSigner, nil
}

func (sshstr Ssh) Connected() (*Auth, *ssh.Client, error) {
	auth, err := ParseStringToBrute(string(sshstr))
	if err != nil {
		fmt.Println(err)
		return nil, nil, err
	}

	methods := []ssh.AuthMethod{}
	if signer, err := sshstr.GetIdRsa(); err == nil {
		methods = append(methods, ssh.PublicKeys(signer))
	} else {
		log.Println("some err:", err)
		fmt.Printf("Now, please type in the password (mandatory): ")
		password, _ := terminal.ReadPassword(0)
		methods = []ssh.AuthMethod{ssh.Password(string(password))}
	}

	if !strings.Contains(auth.Target, ":") {
		auth.Target += ":22"
	}

	sshConfig := &ssh.ClientConfig{
		User: auth.User,
		Auth: methods,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			return nil
		},
		Timeout: time.Second * time.Duration(12),
	}
	client, err := ssh.Dial("tcp", auth.Target, sshConfig)
	if err != nil {
		// log.Println(fmt.Errorf("Failed to dial: %s", err))
		// fmt.Printf("Now, please type in the password (mandatory): ")
		// password, _ := terminal.ReadPassword(0)
		// auth.Pwd = string(password)
		methods = []ssh.AuthMethod{ssh.Password(string(auth.Pwd))}
		sshConfig := &ssh.ClientConfig{
			User: auth.User,
			Auth: methods,
			HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
				return nil
			},
			Timeout: time.Second * time.Duration(12),
		}
		// log.Println(sshConfig)
		client, err = ssh.Dial("tcp", auth.Target, sshConfig)
		if err != nil {
			return nil, nil, err
		}
	}
	return auth, client, nil
}

func (sshstr Ssh) SshShell(cmd string, connectedDo ...func(cli *ssh.Client)) string {
	// rawHost := strings.Split(strings.Split(string(sshstr), "://")[1], "@")[0]
	_, client, err := sshstr.Connected()
	if err != nil {
		return err.Error()
	}
	fmt.Println(datas.Green("---> connect ok"))
	defer func() {
		if connectedDo != nil {
			connectedDo[0](client)
		}
		client.Close()
	}()
	sess, err := client.NewSession()
	if err != nil {
		return err.Error()
	}
	defer sess.Close()

	var b bytes.Buffer
	sess.Stdout = &b
	sess.Stderr = &b
	fmt.Println(datas.Blue("---> running ", cmd))
	if err := sess.Run(cmd); err != nil {
		return err.Error()
	}
	return b.String()
}

func RunByClient(cli *ssh.Client, cmd string) string {
	sess, err := cli.NewSession()
	if err != nil {
		return ""
	}
	defer sess.Close()

	var b bytes.Buffer
	sess.Stdout = &b
	sess.Stderr = &b
	fmt.Println(datas.Blue("---> running "))
	if err := sess.Run(cmd); err != nil {
		log.Println(err)
	}
	return b.String()
}

func (sshstr Ssh) SshSync(file string, ifremove bool, connectedDo ...func(name string, cli *ssh.Client)) (string, string) {
	auth, client, err := sshstr.Connected()
	if err != nil {
		// log.Println(fmt.Errorf("Failed to dial: %s", err))
		return err.Error(), ""
	}
	fmt.Println(datas.Green("---> connect ok"))
	U := uuid.NewMD5(uuid.UUID{}, []byte(file))
	name := filepath.Join("/tmp", U.String())
	defer func() {
		if connectedDo != nil {
			connectedDo[0](name, client)
		}
		client.Close()
	}()

	srcFile, err := os.Open(file)
	if err != nil {
		return err.Error(), ""
	}
	defer srcFile.Close()
	s, _ := srcFile.Stat()
	len := s.Size()

	ftpCli, err := sftp.NewClient(client)
	if err != nil {
		return err.Error(), ""
	}

	fi, err := ftpCli.Stat(name)

	dst := new(sftp.File)
	bar := new(progressbar.ProgressBar)
	bar = progressbar.DefaultBytes(
		len,
		"Upload by sftp",
	)

	if err == nil {
		if ifremove {
			ftpCli.Remove(name)
			// fmt.Println(datas.Blue("remove file then re upload:", name))
			dst, err = ftpCli.Create(name)
			if err != nil {
				return err.Error(), ""
			}
		} else {
			seekOffset := fi.Size()
			if seekOffset < len {
				fmt.Println(datas.Yello("continue with :", seekOffset))
				srcFile.Seek(seekOffset, io.SeekStart)
				dst, err = ftpCli.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND)
				if err != nil {
					return err.Error(), ""
				}
				bar.Add64(seekOffset)
			} else if seekOffset == len {
				fmt.Println(datas.Blue("file uploaded :", name))
				return name, auth.User + ":" + auth.Pwd
			} else {
				ftpCli.Remove(name)
				fmt.Println(datas.Blue("break file re upload:", name))
				dst, err = ftpCli.Create(name)
				if err != nil {
					return err.Error(), ""
				}
			}
		}

	} else {
		fmt.Println(datas.Blue("upload new:", name))
		dst, err = ftpCli.Create(name)
		if err != nil {
			return err.Error(), ""
		}
	}
	if err != nil {
		return err.Error(), ""
	}
	defer dst.Close()

	var out io.Writer
	out = dst

	out = io.MultiWriter(out, bar)
	// datas.Copy(api.con, f)
	io.Copy(out, srcFile)
	ftpCli.Chmod(name, 0x644)
	return name, auth.User + ":" + auth.Pwd
}

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

// func DepAll(targets string, hosts ...string) {
// 	num := len(hosts)
// 	buf, err := ioutil.ReadFile(targets)
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	lines := strings.Split(strings.TrimSpace(string(buf)), "\n")
// 	n_num := len(lines)
// 	per := n_num/num + 1
// 	tmp := []string{}
// 	used := 0
// 	for i, s := range lines {
// 		if used != 0 && used == per {
// 			tmp = append(tmp, s)
// 			used = 0
// 		}
// 	}
// }

type Controller struct {
	BarAll *mpb.Progress
	bars   chan *mpb.Bar
	wait   sync.WaitGroup
}

func NewController() *Controller {
	// var wait sync.WaitGroup
	return &Controller{
		// BarAll: mpb.New(mpb.WithWaitGroup(&wait)),
		BarAll: mpb.New(mpb.WithRefreshRate(800 * time.Millisecond)),
		// wait:   wait,
		bars: make(chan *mpb.Bar, 100),
	}
}
func (bar *Controller) Copy(size int64, outer io.WriteCloser, reader io.ReadCloser, msg string) *mpb.Bar {
	tmpBar := bar.BarAll.AddBar(size,
		mpb.BarStyle("[=>-|"),
		// mpb.BarStyle("╢▌▌░╟"),
		mpb.PrependDecorators(
			decor.CountersKibiByte("% .2f / % .2f"),
			decor.OnComplete(decor.Name(msg, decor.WCSyncSpaceR), "done!"),
			// decor.OnComplete()
		),
		mpb.AppendDecorators(
			decor.EwmaETA(decor.ET_STYLE_GO, 90),
			decor.Name(" "+msg),
			decor.EwmaSpeed(decor.UnitKiB, "% .2f", 60),
		),
	)
	proxyReader := tmpBar.ProxyReader(reader)
	defer proxyReader.Close()
	defer outer.Close()
	io.Copy(outer, proxyReader)
	return tmpBar
}

func (bar *Controller) Upload(connect string, file string, ifremove bool, after ...func(newName string, cli *ssh.Client) string) (err error) {
	defer bar.wait.Done()
	auth, client, err := Ssh(connect).Connected()
	if err != nil {
		return err
	}

	U := uuid.NewMD5(uuid.UUID{}, []byte(file))
	name := filepath.Join("/tmp", U.String())
	defer func() {
		client.Close()
	}()

	//
	srcFile, err := os.Open(file)
	if err != nil {
		return err
	}
	defer srcFile.Close()
	s, _ := srcFile.Stat()
	len := s.Size()
	uploadSize := len
	if err != nil {
		return err
	}

	ftpCli, err := sftp.NewClient(client)
	fi, err := ftpCli.Stat(name)

	dst := new(sftp.File)

	if err == nil {
		if ifremove {
			ftpCli.Remove(name)
			// fmt.Println(datas.Blue("remove file then re upload:", name))
			dst, err = ftpCli.Create(name)
			if err != nil {
				return err
			}
		} else {
			seekOffset := fi.Size()
			if seekOffset < len {
				fmt.Println(datas.Yello("continue with :", seekOffset))
				uploadSize -= seekOffset
				srcFile.Seek(seekOffset, io.SeekStart)
				dst, err = ftpCli.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND)
				if err != nil {
					return err
				}
				// bar.Add64(seekOffset)
			} else if seekOffset == len {
				fmt.Println(datas.Blue("file uploaded :", name))
				// return name, auth.User + ":" + auth.Pwd
			} else {
				ftpCli.Remove(name)
				fmt.Println(datas.Blue("break file re upload:", name))
				dst, err = ftpCli.Create(name)
				if err != nil {
					return err
				}
			}
		}

	} else {
		// fmt.Println(datas.Blue("upload new:", name))
		dst, err = ftpCli.Create(name)
		if err != nil {
			return err
		}
	}
	if err != nil {
		return err
	}
	oneBar := bar.Copy(uploadSize, dst, srcFile, auth.Target)
	if after != nil {
		out := after[0](name, client)
		fmt.Println(out)
		// oneBar.TraverseDecorators
	}
	bar.bars <- oneBar

	return nil

}

func (bar *Controller) RunByClient(cli *ssh.Client, cmd string) string {
	sess, err := cli.NewSession()
	if err != nil {
		return ""
	}
	defer sess.Close()
	var b bytes.Buffer
	sess.Stdout = &b
	sess.Stderr = &b
	fmt.Println(datas.Blue("---> running "))
	if err := sess.Run(cmd); err != nil {
		log.Println(err)
	}
	return b.String()
}

func (bar *Controller) Uploads(file string, hostsFile string, ifremove bool, after ...func(newName string, cli *ssh.Client) string) (err error) {
	buf, err := ioutil.ReadFile(hostsFile)
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
			bar.wait.Add(1)
			go bar.Upload(strings.TrimSpace(l), file, ifremove, after...)
		}
	}
	bar.wait.Wait()

	// bar.BarAll.Wait()

	return
}
