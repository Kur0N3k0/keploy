package main

import (
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/akamensky/argparse"
	"github.com/go-yaml/yaml"
	"github.com/melbahja/goph"
	"github.com/rs/zerolog"
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

func GetLogger() zerolog.Logger {
	logfile, err := os.OpenFile("deploy.log", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0640)
	if err != nil {
		log.Print(err)
	}
	return zerolog.New(
		zerolog.ConsoleWriter{Out: logfile, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()
}

func Deploy(dir string) bool {
	os.Chdir(dir)
	logger := GetLogger()
	logger.Info().Msg("Entered " + dir)

	compose := KeployCompose{}
	cwd, err := os.Getwd()
	if err != nil {
		logger.Err(err)
		return false
	}

	data, err := ioutil.ReadFile("deploy.yml")
	if err != nil {
		logger.Err(err)
		return false
	}
	yaml.Unmarshal(data, &compose)

	auth, err := goph.Key(compose.KeyFile, compose.KeyPass)
	if err != nil {
		logger.Err(err)
		auth = goph.Password(compose.Password)
	}

	callback, err := goph.DefaultKnownHosts()
	if err != nil {
		logger.Err(err)
		return false
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
		logger.Err(err)
		return false
	}
	defer client.Close()

	for i, item := range compose.Files {
		tmp := strings.Split(item, ":")
		if len(tmp) != 2 {
			logger.Error().Msg("invalid files string. Ignore " + item)
			continue
		}
		src, dst := tmp[0], tmp[1]
		logger.Info().Msg("Upload[" + strconv.Itoa(i) + "] " + src + " -> " + dst)
		err := client.Upload(src, dst)
		if err != nil {
			logger.Err(err)
			return false
		}
		logger.Info().Msg("Upload[" + strconv.Itoa(i) + "] Success")
	}

	for _, cmd := range compose.Cmds {
		logger.Info().Msg("CMD: " + cmd)
		res, err := client.Run(cmd)
		if err != nil {
			logger.Err(err)
			continue
		}
		logger.Info().Msg("RES: " + string(res))
	}

	os.Chdir(cwd)
	logger.Info().Msg("Leaved to " + cwd)
	return true
}

func Worker(deploy_paths <-chan string, deploy_result chan<- bool) {
	for deploy_path := range deploy_paths {
		res := Deploy(deploy_path)
		deploy_result <- res
	}
}

func main() {
	logger := zerolog.New(
		zerolog.ConsoleWriter{Out: os.Stderr, TimeFormat: time.RFC3339},
	).Level(zerolog.TraceLevel).With().Timestamp().Caller().Logger()

	parser := argparse.NewParser("keploy", "deploy system using ssh")
	dir := parser.String("d", "dir", &argparse.Options{Required: true, Help: "deploy folder"})
	err := parser.Parse(os.Args)
	if err != nil {
		logger.Err(err)
		os.Exit(1)
	}

	const nJobs = 5
	deploy_paths := make(chan string, nJobs)
	deploy_result := make(chan bool)

	for i := 0; i < nJobs; i++ {
		go Worker(deploy_paths, deploy_result)
	}

	cwd, err := os.Getwd()
	if err != nil {
		logger.Err(err)
		os.Exit(1)
	}
	entrys, err := os.ReadDir(*dir)
	if err != nil {
		logger.Err(err)
		os.Exit(1)
	}

	nResult := 0
	for _, entry := range entrys {
		if entry.IsDir() {
			deploy_path := filepath.Join(cwd, *dir, entry.Name())
			logger.Info().Msg("Start " + deploy_path)
			deploy_paths <- deploy_path
			nResult += 1
		}
	}
	close(deploy_paths)

	for i := 0; i < nResult; i++ {
		<-deploy_result
	}
}
