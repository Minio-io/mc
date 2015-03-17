package main

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path"

	"encoding/json"
	"io/ioutil"
	"os/user"

	"github.com/codegangsta/cli"
	"github.com/minio-io/mc/pkg/s3"
)

const (
	mcConfigDir      = ".minio/mc"
	mcConfigFilename = "config.json"
)

type s3Config struct {
	Auth s3.Auth
}

type mcConfig struct {
	Version string
	S3      s3Config
	Aliases []mcAlias
}

// Global config data loaded from json config file durlng init(). This variable should only
// be accessed via getMcConfig()
var _Config *mcConfig

func getMcConfigDir() string {
	u, err := user.Current()
	if err != nil {
		msg := fmt.Sprintf("mc: Unable to obtain user's home directory. \nERROR[%v]", err)
		fatal(msg)
	}

	return path.Join(u.HomeDir, mcConfigDir)
}

func getMcConfigFilename() string {
	return path.Join(getMcConfigDir(), mcConfigFilename)
}

func getMcConfig() (cfg *mcConfig) {
	if _Config != nil {
		return _Config
	}

	_Config, err := loadMcConfig()
	if err != nil {
		log.Fatalf("mc: Unable to load config file %s. \nERROR[%v]\n", getMcConfigFilename(), err)
	}

	return _Config
}

func loadMcConfig() (config *mcConfig, err error) {
	configBytes, err := ioutil.ReadFile(getMcConfigFilename())
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(configBytes, &config)
	if err != nil {
		return nil, err
	}

	return config, nil
}

// getBashCompletion -
func getBashCompletion() {
	var b bytes.Buffer
	if os.Getenv("SHELL") != "/bin/bash" {
		fatal("Unsupported shell for bash completion detected.. exiting")
	}
	b.WriteString(mcBashCompletion)
	f := getMcBashCompletionFilename()
	fl, err := os.OpenFile(f, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	defer fl.Close()
	_, err = fl.Write(b.Bytes())
	if err != nil {
		fatal(err.Error())
	}
	msg := "\nConfiguration written to " + f
	msg = msg + "\n\n$ source ${HOME}/.minio/mc/mc.bash_completion\n"
	msg = msg + "$ echo 'source ${HOME}/.minio/mc/mc.bash_completion' >> ${HOME}/.bashrc\n"
	info(msg)
}

func parseConfigInput(c *cli.Context) (config *mcConfig, err error) {
	accessKey := c.String("accesskey")
	secretKey := c.String("secretkey")
	config = &mcConfig{
		Version: "0.1.0",
		S3: s3Config{
			Auth: s3.Auth{
				AccessKey:       accessKey,
				SecretAccessKey: secretKey,
			},
		},
		Aliases: []mcAlias{
			{
				Name: "s3",
				URL:  "https://s3.amazonaws.com/",
			},
			{
				Name: "localhost",
				URL:  "http://localhost:9000/",
			},
		},
	}
	return config, nil
}

func getConfig(c *cli.Context) {
	configData, err := parseConfigInput(c)
	if err != nil {
		fatal(err.Error())
	}

	jsonConfig, err := json.MarshalIndent(configData, "", "\t")
	if err != nil {
		fatal(err.Error())
	}

	err = os.MkdirAll(getMcConfigDir(), 0755)
	if err != nil {
		fatal(err.Error())
	}

	configFile, err := os.OpenFile(getMcConfigFilename(), os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	defer configFile.Close()
	if err != nil {
		fatal(err.Error())
	}

	_, err = configFile.Write(jsonConfig)
	if err != nil {
		fatal(err.Error())
	}

	msg := "Configuration written to " + getMcConfigFilename() + "\n"
	info(msg)
}

func doConfig(c *cli.Context) {
	switch true {
	case c.Bool("completion") == true:
		getBashCompletion()
	case c.Bool("completion") == false:
		getConfig(c)
	}
}
