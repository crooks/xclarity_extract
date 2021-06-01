package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/crooks/xclarity_extract/xcapi"
	"github.com/tidwall/gjson"
	"gopkg.in/yaml.v2"
)

var (
	cfg            *Config
	flagConfigFile string
	flagRawOutput  bool
	cpuCodeRE      = regexp.MustCompile(`[\s-]([2568]\d{3})\s`)
)

// Configuration structure
type Config struct {
	API struct {
		BaseURL  string `yaml:"base_url"`
		CertFile string `yaml:"certfile"`
		Password string `yaml:"password"`
		Username string `yaml:"username"`
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
	flag.StringVar(
		&flagConfigFile,
		"config",
		path.Join("/etc/xclarity", "xclarity_extract.yml"),
		"Path to xclarity_extract configuration file",
	)
	flag.BoolVar(
		&flagRawOutput,
		"raw",
		false,
		"Output raw JSON",
	)
	flag.Parse()
}

// parser is the main loop that endlessly fetches URLs and parses them into
// Prometheus metrics
func parser(url string) gjson.Result {
	client := xcapi.NewBasicAuthClient(cfg.API.Username, cfg.API.Password, cfg.API.CertFile)
	bytes, err := client.GetJSON(url)
	if err != nil {
		log.Fatal("Parsing %s returned: %v", url, err)
	}
	return gjson.GetBytes(bytes, "nodeList")
}

// nodeMemory iterates over all the memory modules and returns a total
func nodeMemory(modules gjson.Result) (memory int64) {
	for _, module := range modules.Array() {
		memory += module.Get("capacity").Int()
	}
	return
}

// cpuDesc2Code attempts to extract a meaningful CPU code from the unstructured displayName field
func cpuDesc2Code(desc string) string {
	// Highest priority: Look for a 4 digit Xeon-style CPU code
	res := cpuCodeRE.FindAllStringSubmatch(desc, -1)
	if len(res) == 1 {
		// One match is excellent, return the submatch
		return res[0][1]
	} else if len(res) > 1 {
		// More than a single match suggests Regex refinement is required.
		// Make the bold assumption that the first match is good.
		log.Printf("Multiple CPU code matches: %v", res)
		return res[0][1]
	}
	// No matches found
	return "Unknown"
}

// nodeParser parses the json output from the XClarity API (https://<xclarity_server>/nodes)
func nodeParser(j gjson.Result) {
	for _, jn := range j.Array() {
		sockets := jn.Get("processors").Array()
		// If there aren't any CPUs, we don't want it
		if len(sockets) < 1 {
			continue
		}
		// Name Serial Model CPU Speed Sockets Cores Memory
		fmt.Printf(
			"%-20s %-10s %-10s %-8s %1.2f %1d %2d %4d\n",
			strings.ToLower(jn.Get("name").String()),
			jn.Get("serialNumber").String(),
			jn.Get("model").String(),
			cpuDesc2Code(jn.Get("processors.0.displayName").String()),
			jn.Get("processors.0.speed").Float(),
			len(sockets),
			jn.Get("processors.0.cores").Int(),
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
	j := parser(nodeURL)
	if flagRawOutput {
		fmt.Println(j)
	} else {
		nodeParser(j)
	}
}
