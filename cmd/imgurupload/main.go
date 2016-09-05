package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	ini "gopkg.in/ini.v1"

	"github.com/atotto/clipboard"
	"github.com/fsnotify/fsnotify"
	"github.com/mirtchovski/gosxnotifier"
	homedir "github.com/mitchellh/go-homedir"
	"github.com/parnurzeal/gorequest"
)

const (
	defaultCfg = `[general]
directory = ~/Desktop

[imgur]
client-id = f73c76296263ce7`
)

var (
	cfgPath       string
	cfg           *ini.File
	imgurClientID string
)

type response struct {
	Data struct {
		Link string
	}
	Success bool
	Status  int
}

func init() {
	flag.StringVar(&cfgPath, "c", "~/.imgurupload.cfg", "config file")
}

func main() {
	flag.Parse()

	cfgPath, err := homedir.Expand(cfgPath)
	if err != nil {
		log.Fatal(err)
	}

	cfg, err = ini.Load(cfgPath, []byte(defaultCfg))
	if err != nil {
		log.Fatal(err)
	}

	watchDir, err := homedir.Expand(
		cfg.Section("general").Key("directory").String(),
	)
	if err != nil {
		log.Fatal(err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}

	err = w.Add(watchDir)
	if err != nil {
		log.Fatal(err)
	}

	for {
		select {
		case e := <-w.Events:
			go handleEvent(e)
		case err := <-w.Errors:
			fmt.Println(err)
		}
	}
}

func handleEvent(e fsnotify.Event) {
	if e.Op != fsnotify.Create {
		return
	}

	time.Sleep(time.Second)
	f, err := os.Open(e.Name)
	if err != nil {
		log.Println(err)
		return
	}

	var b bytes.Buffer
	enc := base64.NewEncoder(base64.StdEncoding, &b)
	_, err = io.Copy(enc, f)
	if err != nil {
		log.Println(err)
		return
	}

	_, bb, errs := gorequest.New().
		Post("https://api.imgur.com/3/image").
		Set("Authorization", fmt.Sprintf("Client-ID %s", cfg.Section("imgur").Key("client-id"))).
		SendStruct(map[string]string{"image": b.String()}).
		EndBytes()

	if len(errs) > 0 {
		for _, err := range errs {
			log.Println(err)
		}

		return
	}

	var res response
	err = json.Unmarshal(bb, &res)
	if err != nil {
		log.Println(err)
		return
	}

	if res.Success {
		n := gosxnotifier.NewNotification(res.Data.Link)
		n.Link = res.Data.Link
		n.Push()
		clipboard.WriteAll(res.Data.Link)
	}
}
