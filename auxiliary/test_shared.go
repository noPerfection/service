package auxiliary

import (
	"github.com/noPerfection/datatype/data_type/key_value"
	"github.com/noPerfection/os/arg"
	"github.com/noPerfection/os/path"
	"github.com/noPerfection/protocol/client"
	clientConfig "github.com/noPerfection/protocol/client/config"
	"github.com/noPerfection/protocol/handler/base"
	handlerConfig "github.com/noPerfection/protocol/handler/config"
	serviceLib "github.com/noPerfection/service"
	"github.com/noPerfection/service/flag"
	"github.com/pebbe/zmq4"
	"gopkg.in/yaml.v3"
	win "os"
	"path/filepath"
)

// ParentConfig returns parent config as a struct and string
func ParentConfig(parentId string, parentUrl string, port uint64) (*clientConfig.Client, string, error) {
	// Creating a proxy with the valid flags must succeed
	parentClient := clientConfig.New(parentUrl, parentId, port, zmq4.REP)
	parentKv, err := key_value.NewFromInterface(parentClient)
	if err != nil {
		return nil, "", err
	}
	parentStr := parentKv.String()
	return parentClient, parentStr, nil
}

func DeleteLastFlags(amount int) {
	win.Args = win.Args[:len(win.Args)-amount]
}

func NewParent(id, url, category string,
	handler base.Interface) (*serviceLib.Service, error) {
	win.Args = append(win.Args, arg.NewFlag(flag.IdFlag, id), arg.NewFlag(flag.UrlFlag, url))

	created, err := serviceLib.New()
	if err != nil {
		return nil, err
	}

	created.SetHandler(category, handler)

	win.Args = win.Args[:len(win.Args)-2]

	return created, nil
}

// CloseParent dir could be a currentDir
func CloseParent(parent *serviceLib.Service, dir string) error {
	if err := parent.Context().Close(); err != nil {
		return err
	}

	return DeleteYaml(dir, "app")
}

func CreateYaml(dir, name string) error {
	kv := key_value.New().Set("services", []interface{}{})

	marshalledConfig, err := yaml.Marshal(kv.Map())
	if err != nil {
		return err
	}

	filePath := filepath.Join(dir, name+".yml")

	f, err := win.OpenFile(filePath, win.O_RDWR|win.O_CREATE|win.O_TRUNC, 0644)
	if err != nil {
		return err
	}
	_, err = f.Write(marshalledConfig)
	if err != nil {
		return err
	}

	if err := f.Close(); err != nil {
		return err
	}

	return nil
}

func DeleteYaml(dir, name string) error {
	filePath := filepath.Join(dir, name+".yml")

	exist, err := path.FileExist(filePath)
	if err != nil {
		return err
	}

	if !exist {
		return nil
	}

	return win.Remove(filePath)
}

func MainHandler(s *serviceLib.Service) base.Interface {
	return s.Handlers["main"].(base.Interface)
}

func ExternalClient(url string, hConfig *handlerConfig.Handler) (*client.Socket, error) {
	// let's test that handler runs
	targetZmqType := handlerConfig.SocketType(hConfig.Type)
	externalConfig := clientConfig.New(url, hConfig.Id, hConfig.Port, targetZmqType)
	externalConfig.UrlFunc(clientConfig.Url)
	externalClient, err := client.New(externalConfig)
	return externalClient, err
}

func ManagerClient(s *serviceLib.Service) (*client.Socket, error) {
	createdConfig, err := s.Context().Config().Service(s.Id())
	if err != nil {
		return nil, err
	}
	managerConfig := createdConfig.Manager
	managerConfig.UrlFunc(clientConfig.Url)
	managerClient, err := client.New(managerConfig)
	return managerClient, err
}
