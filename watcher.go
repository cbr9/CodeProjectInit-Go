package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strings"

	"github.com/rjeczalik/notify"
)

const configFile = "./config.json"

func main() {
	if runtime.GOOS == "windows" {
		log.Fatal(errors.New("windows is not supported"))
		return
	}
	watcher := newWatcher()
	watcher.watch()

}

type watcher struct {
	home   string
	code   string
	config map[string]folderConfig
}

type folderConfig struct {
	Depth        int      `json:"depth"`
	ExcludedDirs []string `json:"excluded_dirs"`
}

func newWatcher() *watcher {
	data, err := ioutil.ReadFile(configFile)
	if err != nil {
		log.Fatal(err)
		return nil
	}
	var config map[string]folderConfig
	err = json.Unmarshal(data, &config)
	if err != nil {
		fmt.Println(err)
	}

	home := os.Getenv("HOME")
	return &watcher{
		home:   home,
		code:   path.Join(home, "Code"),
		config: config,
	}
}

// From the official documentation:
// "Running git init in an existing repository is safe. It will not overwrite things that are already there".
// This means we don't need to check if it already exists
func (w *watcher) runInitCmd(newFolder string, command string) {
	err := os.Chdir(newFolder)
	if err != nil {
		log.Println(err)
		return
	}
	cmdPieces := strings.Split(command, " ")
	name := cmdPieces[0]
	args := cmdPieces[1:]
	cmd := exec.Command(name, args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	err = cmd.Run()
	if err != nil {
		log.Println(err)
	}
}

func isUnixHiddenDir(name string) bool {
	return strings.HasPrefix(name, ".")
}

func contains(slice []string, elements ...string) bool {
	for i := range slice {
		for _, elem := range elements {
			if slice[i] == elem {
				return true
			}
		}
	}
	return false
}

func (w *watcher) watch() {
	_ = os.Chdir(w.code)
	c := make(chan notify.EventInfo, 1)
	err := notify.Watch("./...", c, notify.Create)
	if err != nil {
		log.Fatal(err)
	}
	defer notify.Stop(c)

	for {
		change := <-c
		switch change.Event() {
		case notify.Create:
			fi, err := os.Stat(change.Path())
			if err != nil {
				log.Println(err)
				continue
			}
			if fi.IsDir() {
				pathChunks := strings.Split(strings.TrimPrefix(change.Path(), w.code+"/"), "/")
				topParent := pathChunks[0]
				if len(pathChunks) == w.config[topParent].Depth && !contains(pathChunks, w.config[topParent].ExcludedDirs...) && !isUnixHiddenDir(path.Base(change.Path())) {
					// avoid hidden folders
					switch topParent {
					case "Go":
						w.runInitCmd(change.Path(), "go mod init")
						w.runInitCmd(change.Path(), "git init")
						break
					case "Rust":
						w.runInitCmd(change.Path(), "cargo init")
						break
					default:
						w.runInitCmd(change.Path(), "git init")
					}
				}
			}
		}
	}
}
