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
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fatih/color"
	"github.com/google/uuid"

	"gitee.com/dark.H/go-remote-repl/datas"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/terminal"

	"github.com/vbauerster/mpb/v5"
	"github.com/vbauerster/mpb/v5/decor"
)

var (
	UploadModeSplit  = 1
	UploadModeSingle = 0
	REMOTE_TMP       = "/tmp/_deploys_files"
	Green            = color.New(color.FgGreen).SprintFunc()
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
	if strings.Contains(s, "##") {
		s = strings.TrimSpace(strings.SplitN(s, "##", 2)[0])
	}
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

// func (sshstr Ssh) SshShell(cmd string, connectedDo ...func(cli *ssh.Client)) string {
// 	// rawHost := strings.Split(strings.Split(string(sshstr), "://")[1], "@")[0]
// 	_, client, err := sshstr.Connected()
// 	if err != nil {
// 		return err.Error()
// 	}
// 	fmt.Println(datas.Green("---> connect ok"))
// 	defer func() {
// 		if connectedDo != nil {
// 			connectedDo[0](client)
// 		}
// 		client.Close()
// 	}()
// 	sess, err := client.NewSession()
// 	if err != nil {
// 		return err.Error()
// 	}
// 	defer sess.Close()

// 	var b bytes.Buffer
// 	sess.Stdout = &b
// 	sess.Stderr = &b
// 	// fmt.Println(datas.Blue("---> running ", cmd))
// 	if err := sess.Run(cmd); err != nil {
// 		return err.Error()
// 	}
// 	return b.String()
// }

func RunByClient(cli *ssh.Client, cmd string) string {
	sess, err := cli.NewSession()
	if err != nil {
		return ""
	}
	defer sess.Close()

	var b bytes.Buffer
	sess.Stdout = &b
	sess.Stderr = &b
	// fmt.Println(datas.Blue("---> running "))
	if err := sess.Run(cmd); err != nil {
		log.Println(err)
	}
	return b.String()
}

// func (sshstr Ssh) SshSync(file string, ifremove bool, connectedDo ...func(name string, cli *ssh.Client)) (string, string) {
// 	auth, client, err := sshstr.Connected()
// 	if err != nil {
// 		// log.Println(fmt.Errorf("Failed to dial: %s", err))
// 		return err.Error(), ""
// 	}
// 	fmt.Println(datas.Green("---> connect ok"))
// 	U := uuid.NewMD5(uuid.UUID{}, []byte(file))
// 	name := filepath.Join("/tmp", U.String())
// 	defer func() {
// 		if connectedDo != nil {
// 			connectedDo[0](name, client)
// 		}
// 		client.Close()
// 	}()

// 	srcFile, err := os.Open(file)
// 	if err != nil {
// 		return err.Error(), ""
// 	}
// 	defer srcFile.Close()
// 	s, _ := srcFile.Stat()
// 	len := s.Size()

// 	ftpCli, err := sftp.NewClient(client)
// 	if err != nil {
// 		return err.Error(), ""
// 	}

// 	fi, err := ftpCli.Stat(name)

// 	dst := new(sftp.File)
// 	bar := new(progressbar.ProgressBar)
// 	bar = progressbar.DefaultBytes(
// 		len,
// 		"Upload by sftp",
// 	)

// 	if err == nil {
// 		if ifremove {
// 			ftpCli.Remove(name)
// 			// fmt.Println(datas.Blue("remove file then re upload:", name))
// 			dst, err = ftpCli.Create(name)
// 			if err != nil {
// 				return err.Error(), ""
// 			}
// 		} else {
// 			seekOffset := fi.Size()
// 			if seekOffset < len {
// 				fmt.Println(datas.Yello("continue with :", seekOffset))
// 				srcFile.Seek(seekOffset, io.SeekStart)
// 				dst, err = ftpCli.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND)
// 				if err != nil {
// 					return err.Error(), ""
// 				}
// 				bar.Add64(seekOffset)
// 			} else if seekOffset == len {
// 				fmt.Println(datas.Blue("file uploaded :", name))
// 				return name, auth.User + ":" + auth.Pwd
// 			} else {
// 				ftpCli.Remove(name)
// 				fmt.Println(datas.Blue("break file re upload:", name))
// 				dst, err = ftpCli.Create(name)
// 				if err != nil {
// 					return err.Error(), ""
// 				}
// 			}
// 		}

// 	} else {
// 		fmt.Println(datas.Blue("upload new:", name))
// 		dst, err = ftpCli.Create(name)
// 		if err != nil {
// 			return err.Error(), ""
// 		}
// 	}
// 	if err != nil {
// 		return err.Error(), ""
// 	}
// 	defer dst.Close()

// 	var out io.Writer
// 	out = dst

// 	out = io.MultiWriter(out, bar)
// 	// datas.Copy(api.con, f)
// 	io.Copy(out, srcFile)
// 	ftpCli.Chmod(name, 0x644)
// 	return name, auth.User + ":" + auth.Pwd
// }

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
	BarAll           *mpb.Progress
	outputs          chan string
	wait             sync.WaitGroup
	UploadMode       int
	UploadFileName   string
	UploadFilePrefix string
}

func NewController() *Controller {
	// var wait sync.WaitGroup
	return &Controller{
		// BarAll: mpb.New(mpb.WithWaitGroup(&wait)),
		BarAll: mpb.New(mpb.WithRefreshRate(800 * time.Millisecond)),
		// wait:   wait,
		outputs: make(chan string, 256),
	}
}
func (bar *Controller) Copy(size int64, outer io.WriteCloser, reader io.ReadCloser, msg string) *mpb.Bar {
	tmpBar := bar.BarAll.AddBar(size,
		mpb.BarStyle("[=>-|"),
		mpb.BarRemoveOnComplete(),
		// mpb.BarFillerClearOnComplete(),
		// mpb.BarStyle("╢▌▌░╟"),
		mpb.PrependDecorators(
			decor.CountersKibiByte("% .2f / % .2f"),
			decor.OnComplete(decor.Name(msg+" uploading", decor.WCSyncSpaceR), " installinng"),
			// decor.OnComplete()
		),
		mpb.AppendDecorators(
			decor.Percentage(decor.WC{W: 5}),
			// decor.EwmaETA(decor.ET_STYLE_GO, 90),
		// 	// decor.Name(" "+msg),
		// 	decor.EwmaSpeed(decor.UnitKiB, "% .2f", 60),
		),
	)
	proxyReader := tmpBar.ProxyReader(reader)
	defer proxyReader.Close()
	defer outer.Close()
	io.Copy(outer, proxyReader)
	return tmpBar
}

func (bar *Controller) ListenAndPrint() {
	for {
		b := <-bar.outputs
		if strings.Contains(b, "[SEP]") {
			fs := strings.SplitN(b, "[SEP]", 2)
			fmt.Fprintf(os.Stderr, "[Fin] %s\n", Green(fs[0]))
			o := strings.TrimSpace(fs[1])
			if o != "" {
				fmt.Println(o)
			}
		} else {
			o := strings.TrimSpace(b)
			if o != "" {
				fmt.Println(o)
			}
		}
	}
}

func (bar *Controller) OnlyRun(hosts []string, after ...func(cli *ssh.Client) string) (err error) {
	go bar.ListenAndPrint()
	for _, l := range hosts {
		if strings.TrimSpace(l) != "" {
			bar.wait.Add(1)
			go bar.RunShell(strings.TrimSpace(l), after...)
		}
	}
	bar.wait.Wait()
	return
}

func (bar *Controller) RunShell(connect string, after ...func(cli *ssh.Client) string) (err error) {
	defer bar.wait.Done()
	_, client, err := Ssh(connect).Connected()
	if err != nil {
		fmt.Println(datas.Red("Connected Failed ", connect))
		return err
	}
	defer func() {
		client.Close()
	}()
	if after != nil {
		bar.outputs <- after[0](client)
	}
	return
}

func (bar *Controller) Upload(connect string, file string, ifremove bool, after ...func(newName string, cli *ssh.Client) string) (err error) {
	defer bar.wait.Done()
	auth, client, err := Ssh(connect).Connected()
	if err != nil {
		ss, _ := ParseStringToBrute(connect)
		fmt.Println(datas.Red("Connected Failed ", ss))
		return err
	}
	U := uuid.NewMD5(uuid.UUID{}, []byte(file))
	name := filepath.Join(REMOTE_TMP, U.String())

	if bar.UploadFileName != "" {
		name = filepath.Join(REMOTE_TMP, bar.UploadFileName)
	}
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
	length := s.Size()
	uploadSize := length
	if err != nil {
		return err
	}

	ftpCli, err := sftp.NewClient(client)
	ftpCli.Mkdir(REMOTE_TMP)
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
			if seekOffset < length {
				fmt.Println(datas.Yello("continue with :", seekOffset))
				uploadSize -= seekOffset
				srcFile.Seek(seekOffset, io.SeekStart)
				dst, err = ftpCli.OpenFile(name, os.O_RDWR|os.O_CREATE|os.O_APPEND)
				if err != nil {
					return err
				}
				// bar.Add64(seekOffset)
			} else if seekOffset == length {
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
	if after == nil {
		bar.Copy(uploadSize, dst, srcFile, auth.Target+" new file: "+name)
		// oneBar.Abort()
	} else {
		bar.Copy(uploadSize, dst, srcFile, auth.Target)
		if len(after) > 0 && after[0] != nil {
			bar.outputs <- after[0](name, client)
		}
		// oneBar.Abort(true)
	}
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
	// fmt.Println(datas.Blue("---> running "))
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
	go bar.ListenAndPrint()
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

func (bar *Controller) SetSplitUpload(name string, prefix ...string) {
	bar.UploadMode = UploadModeSplit
	bar.UploadFileName = filepath.Base(name)
	if prefix != nil {
		bar.UploadFilePrefix = prefix[0]
	}
}

func (bar *Controller) SetSingleUpload(name string) {
	bar.UploadMode = UploadModeSingle
	bar.UploadFileName = filepath.Base(name)
}

func (bar *Controller) UploadsByHosts(hosts []string, file string, ifremove bool, after ...func(newName string, cli *ssh.Client) string) (err error) {
	if bar.UploadMode == UploadModeSingle {
		for _, l := range hosts {
			if strings.TrimSpace(l) != "" {
				bar.wait.Add(1)
				go bar.Upload(strings.TrimSpace(l), file, ifremove, after...)
			}
		}
	} else if bar.UploadMode == UploadModeSplit {
		lenHost := len(hosts)
		rawBuf, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatal(err)
		}
		lines := strings.Split(string(rawBuf), "\n")
		perLines := len(lines) / lenHost
		// if len(lines)%lenHost != 0 {
		// 	perLines++
		// }
		oneFileLines := []string{}
		oldFileType := strings.Split(file, ".")
		tmpDir := os.TempDir()
		nowUseHost := 0
		endToRemove := []string{}
		for i, l := range lines {
			l := strings.TrimSpace(l)
			if l == "" {
				continue
			}
			if i%perLines == 0 && len(oneFileLines) > 0 {

				tmpFile := filepath.Join(tmpDir, fmt.Sprintf("tmp-parm-%d", i))
				if len(oldFileType) > 0 {
					tmpFile += "." + oldFileType[len(oldFileType)-1]
				}
				if err := ioutil.WriteFile(tmpFile, []byte(bar.UploadFilePrefix+"\n"+strings.Join(oneFileLines, "\n")), os.ModePerm); err == nil {
					bar.wait.Add(1)
					go bar.Upload(strings.TrimSpace(hosts[nowUseHost]), tmpFile, ifremove, after...)
					endToRemove = append(endToRemove, tmpFile)
					oneFileLines = []string{}
					nowUseHost++
					nowUseHost %= lenHost
				} else {
					log.Fatal("err in create one tmp file to upload:", err)
				}

			}
			oneFileLines = append(oneFileLines, l)
		}

		if len(oneFileLines) > 0 {

			tmpFile := filepath.Join(tmpDir, "tmp-parm-end")
			if len(oldFileType) > 0 {
				tmpFile += "." + oldFileType[len(oldFileType)-1]
			}
			if err := ioutil.WriteFile(tmpFile, []byte(strings.Join(oneFileLines, "\n")), os.ModePerm); err == nil {
				bar.wait.Add(1)
				go bar.Upload(strings.TrimSpace(hosts[nowUseHost]), tmpFile, ifremove, after...)
				endToRemove = append(endToRemove, tmpFile)
				oneFileLines = []string{}
				nowUseHost++
			} else {
				log.Fatal("err in create one tmp file to upload:", err)
			}
		}

		defer func() {
			for _, i := range endToRemove {
				os.Remove(i)
			}
		}()

	}

	bar.wait.Wait()
	// bar.BarAll.Wait()

	return
}

func ReadHostsFile(f string) (hs []string) {
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
			hs = append(hs, l)
		}
	}
	return hs
}
