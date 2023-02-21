package main

import (
	"encoding/json"
	"io/ioutil"
	"log"
	"os"

	batchnotify "github.com/unkaktus/batch-notify"
)

func main() {
	config := &batchnotify.Config{}
	configFile, err := os.Open("config.json")
	if err != nil {
		log.Fatalf("open config file: %v", err)
	}
	data, err := ioutil.ReadAll(configFile)
	if err != nil {
		log.Fatalf("read config file: %v", err)
	}

	if err := json.Unmarshal(data, config); err != nil {
		log.Fatalf("unmarshal JSON: %v", err)
	}

	if err := batchnotify.Run(config); err != nil {
		log.Fatal(err)
	}
}
