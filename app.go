package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strings"

	"github.com/go-yaml/yaml"
	"github.com/melbahja/goph"
)

type KeployCompose struct {
	User     string
	Host     string
	Port     uint
	KeyFile  string
	KeyPass  string
	Password string
	Files    []string
	Cmds     []string
}

func main() {
	compose := KeployCompose{}
	data, err := ioutil.ReadFile(os.Args[1])
	if err != nil {
		log.Fatal(err)
	}
	yaml.Unmarshal(data, &compose)

	auth, err := goph.Key(compose.KeyFile, compose.KeyPass)
	if err != nil {
		auth = goph.Password(compose.Password)
	}

	callback, err := goph.DefaultKnownHosts()
	if err != nil {
		log.Fatal(err)
	}

	client, err := goph.NewConn(&goph.Config{
		User:     compose.User,
		Addr:     compose.Host,
		Port:     compose.Port,
		Auth:     auth,
		Timeout:  goph.DefaultTimeout,
		Callback: callback,
	})
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	for _, item := range compose.Files {
		tmp := strings.Split(item, ":")
		if len(tmp) != 2 {
			log.Fatal("invalid files string")
		}
		src, dst := tmp[0], tmp[1]
		err := client.Upload(src, dst)
		if err != nil {
			log.Fatal(err)
		}
	}

	for _, cmd := range compose.Cmds {
		fmt.Println(cmd)
		xmd, err := client.Command("bash", "-c", cmd)
		if err != nil {
			log.Fatal(err)
		}
		err = xmd.Run()
		if err != nil {
			log.Fatal(err)
		}
	}
}
