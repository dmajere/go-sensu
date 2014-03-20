package main

import (
	"flag"
	"os"
	"os/signal"
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

func main() {
	flagSet.Parse(os.Args[1:])

	exitChan := make(chan int)
	signalChan := make(chan os.Signal, 1)
	go func() {
		<-signalChan
		exitChan <- 1
	}()
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	opts := NewSensuOptions(flagSet)
	sensu := Sensu(opts)

	sensu.Start()
	<-exitChan
	//sensu.Exit()
}
