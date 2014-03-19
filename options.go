package main

import (

	"log"
	"flag"
	"io/ioutil"
"encoding/json"
)

type rabbitMQOptions struct {
        SSL     rabbitSSLOptions `json:"ssl"`
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

type sensuCheck struct {
	    // Base Fields
        Name string `json:"name"`
        Issued int64 `json:"issued"`
        Command string `json:"command"`

        //Fields that will be in response
        Executed int64 `json:"executed"`
        Output string `json:"output"`
        Status uint8 `json:"status"`
        Duration string `json:"duration"`
		Interval uint64
		Occurrences int64
		Refresh uint64
		Subscribers []string

}
type CheckResult struct {
        Client string `json:"client"`
        Check *sensuCheck  `json:"check"`
}

type ClientKeepalive struct {
        Thresholds ClientKeepaliveThresholds `json:"thresholds"`
        Handler string `json:"handler"`
}

type ClientKeepaliveThresholds struct {
        Warning uint8 `json:"warning"`
        Critical uint8 `json:"critical"`
}

type sensuClient struct {
        Name string `json:"name"`
        Address string `json:"address"`
        Subscriptions []string `json:"subscriptions"`
        Keepalive ClientKeepalive `json:"keepalive"`
        Timestamp int64  `json:"timestamp"`
}

type sensuOptions struct {
	Rabbitmq 	rabbitMQOptions
	Client   	sensuClient
	Checks		[]sensuCheck
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

