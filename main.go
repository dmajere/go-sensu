package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"os/user"
	"strconv"
	"syscall"
)

var (
	flagSet = flag.NewFlagSet("sensu", flag.ExitOnError)

	config     = flagSet.String("config", "/etc/sensu/config.json", "Sensu JSON config FILE")
	config_dir = flagSet.String("config_dir", "/etc/sensu/conf.d/", "DIR or comma-delimited DIR list for Sensu JSON config files")
	logfile    = flagSet.String("logfile", "/tmp/sensu-client", "Log to a given FILE")
	verbose    = flagSet.Bool("verbose", false, "Enable verbose logging")
	pid_file   = flagSet.String("pid_file", "/var/run/sensu/sensu-client.pid", "Write the PID to a given FILE")
)

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

func main() {
	flagSet.Parse(os.Args[1:])

	u, err := user.Lookup("sensu")
	if err != nil {
		log.Fatal("No Such User: sensu")
	}

	uid, err := strconv.ParseInt(u.Uid, 10, 0)
	if err != nil {
		log.Fatal("Cant Get User Uid")
	}
	gid, err := strconv.ParseInt(u.Gid, 10, 0)
	if err != nil {
		log.Fatal("Cant Get User Gid")
	}
	syscall.Setuid(int(uid))
	syscall.Setgid(int(gid))
	syscall.Setenv("HOME", u.HomeDir)
	syscall.Setenv("LOGNAME", u.Name)

	exitChan := make(chan int)
	signalChan := make(chan os.Signal, 1)
	go func() {
		<-signalChan
		fmt.Println("Got Signal")
		exitChan <- 1
	}()
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	opts := NewSensuOptions(flagSet)
	sensu := Sensu(opts)

	sensu.Start()
	<-exitChan
	sensu.Exit()
}
