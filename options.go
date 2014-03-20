package main

import (
	"encoding/json"
	"flag"
	"io/ioutil"
	"log"
)

type rabbitMQOptions struct {
	SSL      rabbitSSLOptions `json:"ssl"`
	Port     int
	Host     string
	User     string
	Password string
	Vhost    string
}

type rabbitSSLOptions struct {
	Private_key_file string
	Cert_chain_file  string
}


type ClientKeepalive struct {
	Thresholds ClientKeepaliveThresholds `json:"thresholds"`
	Handler    string                    `json:"handler"`
}

type ClientKeepaliveThresholds struct {
	Warning  uint8 `json:"warning"`
	Critical uint8 `json:"critical"`
}

type sensuClient struct {
	Name          string          `json:"name"`
	Address       string          `json:"address"`
	Subscriptions []string        `json:"subscriptions"`
	Keepalive     ClientKeepalive `json:"keepalive"`
	Timestamp     int64           `json:"timestamp"`
}

type sensuOptions struct {
	Rabbitmq rabbitMQOptions
	Client   sensuClient
	Checks   []sensuCheck
}

func NewSensuOptions(flagSet *flag.FlagSet) *sensuOptions {

	var options *sensuOptions
	var configFiles []string

	configFiles = append(configFiles, flagSet.Lookup("config").Value.String())
	configDir := flagSet.Lookup("config_dir").Value.String()
	configFiles = append(configFiles, Walk(configDir)...)

	for _, config := range configFiles {
		jsonStream, err := ioutil.ReadFile(config)

		if err != nil {
			log.Fatal(err)
		}
		json.Unmarshal(jsonStream, &options)
	}

	return options
}
