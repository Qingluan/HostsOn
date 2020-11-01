package hosts

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/google/uuid"
	"golang.org/x/crypto/ssh"
)

type API struct {
	Exec string
	Args []string
}

func (api *API) ID() string {
	U := uuid.NewMD5(uuid.UUID{}, []byte(api.Exec))
	return U.String()
}

func (api *API) RemotePath(otherDir ...string) string {
	if otherDir == nil {
		return filepath.Join("/tmp", api.ID())
	}
	return filepath.Join(otherDir[0], api.ID())
}

// if return id > 0 is running
func (api *API) GetRemoteUID(cli *ssh.Client) int {
	res := RunByClient(cli, fmt.Sprintf("ps aux | grep %s | egrep -v \"(grep|egrep)\" | awk '{print $2}' |xargs ", api.ID()))
	if strings.TrimSpace(res) == "" {
		return -1
	}
	e, err := strconv.Atoi(strings.TrimSpace(res))
	if err != nil {
		return -1
	}
	return e
}

func (api *API) Run() int {
	return -1
}
