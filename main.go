package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"
)

var (
	cfg            *Config
	flagConfigFile string
)

// Configuration structure
type Config struct {
	API struct {
		BaseURL  string        `yaml:"base_url"`
		CertFile string        `yaml:"certfile"`
		Interval time.Duration `yaml:"interval"`
		Password string        `yaml:"password"`
		Username string        `yaml:"username"`
	}
}

func newConfig(filename string) (*Config, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	config := &Config{}
	if err := d.Decode(&config); err != nil {
		return nil, err
	}
	return config, nil
}

func parseFlags() {
	home, err := os.UserHomeDir()
	if err != nil {
		log.Fatal("Unable to determine user's homedir")
	}
	flag.StringVar(
		&flagConfigFile,
		"config",
		path.Join(home, "xclarity_extract.yml"),
		"Path to xclarity_extract configuration file",
	)
	flag.Parse()
}

// parser is the main loop that endlessly fetches URLs and parses them into
// Prometheus metrics
func parser(url string) {
	client := newBasicAuthClient(cfg.API.Username, cfg.API.Password)
	j, err := client.getJSON(url, "nodeList")
	if err != nil {
		log.Printf("Parsing %s returned: %v", url, err)
	} else {
		nodeParser(j)
	}
}

// nodeMemory iterates over all the memory modules and returns a total
func nodeMemory(modules gjson.Result) (memory int64) {
	for _, module := range modules.Array() {
		memory += module.Get("capacity").Int()
	}
	return
}

// nodeParser parses the json output from the XClarity API (https://<xclarity_server>/nodes)
func nodeParser(j gjson.Result) {
	for _, jn := range j.Array() {
		sockets := jn.Get("processors").Array()
		// If there aren't any CPUs, we don't want it
		if len(sockets) < 1 {
			continue
		}
		cores := jn.Get("processors.0.cores").Int()
		fmt.Printf(
			"%s,%s,%s,%d,%d,%d\n",
			strings.ToLower(jn.Get("name").String()),
			jn.Get("serialNumber").String(),
			jn.Get("model").String(),
			len(sockets),
			cores,
			nodeMemory(jn.Get("memoryModules")),
		)
	}
}

func main() {
	var err error
	parseFlags()
	cfg, err = newConfig(flagConfigFile)
	if err != nil {
		log.Fatalf("Unable to parse config file: %v", err)
	}
	nodeURL := fmt.Sprintf("%s/nodes", cfg.API.BaseURL)
	parser(nodeURL)
}
