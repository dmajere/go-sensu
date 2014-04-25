package main

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"github.com/streadway/amqp"
	"log"
	"net/url"
	"sync/atomic"
	"time"
)

type sensu struct {
	options *sensuOptions
	client  *sensuClient

	rabbitAddr              *url.URL
	rabbitCfg               *tls.Config
	rabbitConn              *amqp.Connection
	rabbitChan              *amqp.Channel
	rabbitConnectionClose   chan int
	rabbitConnectionRestart chan int

	stopServe chan int
	exitChan  chan int
}

func Sensu(opt *sensuOptions) *sensu {

	rabbitCfg := new(tls.Config)
	rabbitCfg.RootCAs = x509.NewCertPool()
	cert, err := tls.X509KeyPair(ReadFile(opt.Rabbitmq.SSL.Cert_chain_file), ReadFile(opt.Rabbitmq.SSL.Private_key_file))
	if err != nil {
		log.Fatal(err)
	}
	rabbitCfg.Certificates = append(rabbitCfg.Certificates, cert)
	rabbitCfg.InsecureSkipVerify = true

	rabbitAddr := url.URL{
		Scheme: "amqps",
		Host:   fmt.Sprintf("%s:%d", opt.Rabbitmq.Host, opt.Rabbitmq.Port),
		Path:   "/" + opt.Rabbitmq.Vhost,
		User:   url.UserPassword(opt.Rabbitmq.User, opt.Rabbitmq.Password),
	}

	s := &sensu{

		options:                 opt,
		client:                  &opt.Client,
		rabbitAddr:              &rabbitAddr,
		rabbitCfg:               rabbitCfg,
		rabbitConnectionClose:   make(chan int),
		rabbitConnectionRestart: make(chan int),
		stopServe:               make(chan int),
		exitChan:                make(chan int),
	}

	s.client.Keepalive.Thresholds.Warning = 120
	s.client.Keepalive.Thresholds.Critical = 180

	return s
}

func (s *sensu) Start() {
	log.Println("Start Monitoring")
	s.setupRabbit()
	s.ConsumeAndServe()
	//go s.standaloneSetup()
	go s.KeepAlive()
}

func (s *sensu) Exit() {
	log.Println("Stop Monitoring")
	s.stopServe <- 1
	s.rabbitConnectionClose <- 1
	time.Sleep(5 * time.Second)
}

func (s *sensu) setupRabbit() {
	var syncReconnect int32
	atomic.StoreInt32(&syncReconnect, 0)
	setupRabbit(s)
	go func() {

		for {
			select {
			case <-s.rabbitConnectionClose:
				for {
					if atomic.LoadInt32(&syncReconnect) == 0 {
						atomic.StoreInt32(&syncReconnect, 1)
						log.Println("Close RabbitMQ Connection")
						s.rabbitConn.Close()
						break
					}
				}
				break
			case <-s.rabbitConnectionRestart:
				if atomic.LoadInt32(&syncReconnect) == 0 {
					atomic.StoreInt32(&syncReconnect, 1)
					setupRabbit(s)
					time.Sleep(5 * time.Second)
					atomic.StoreInt32(&syncReconnect, 0)
				}
			}
		}
	}()
}

func (s *sensu) ConsumeAndServe() {
	c := s.Consume()
	go s.Serve(c)
}

func (s *sensu) Consume() <-chan amqp.Delivery {
	ch, err := s.rabbitChan.Consume("", s.client.Name, false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}
	return ch
}

func (s *sensu) Serve(checks <-chan amqp.Delivery) {
	for {
		select {
		case <-s.stopServe:
			log.Println("Stop Consuming New Checks")
			break
		case msg, ok := <-checks:
			if !ok {
				s.rabbitConnectionRestart <- 1
			} else {
				go s.handleCheck(&msg)
			}
		}
	}
}

func (s *sensu) handleCheck(msg *amqp.Delivery) {
	var check *sensuCheckRemote
	err := json.Unmarshal(msg.Body, &check)
	if err != nil {
		log.Fatal(err)
	}
	check.Execute()
	log.Printf("Exit Status: %d; Output: %s", check.Status, check.Output)
	result := &RemoteResult{
		Client: s.client.Name,
		Check:  check,
	}

	body, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("json encoder: %s", err)
	}
	s.Publish("results", body)
}

func (s *sensu) Publish(exchange string, message []byte) {
	msg := amqp.Publishing{
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now(),
		ContentType:  "text/plain",
		Body:         message,
	}
	for {
		err := s.rabbitChan.Publish(exchange, "", false, false, msg)
		if err != nil {
			log.Printf("Channel Publish Error: %s. Reconnect...", err)
			s.rabbitConnectionRestart <- 1
			continue

		}
		break
	}
}

func (s *sensu) standaloneSetup() {

	for _, check := range s.options.Checks {
		go func() {

			for {

				check.Execute()

				result := &LocalResult{
					Client: s.client.Name,
					Check:  &check,
				}

				body, err := json.Marshal(result)
				if err != nil {
					log.Fatalf("json encoder: %s", err)
				}

				s.Publish("results", body)

				time.Sleep(time.Duration(check.Interval) * time.Second)
			}
		}()
	}
}

func (s *sensu) KeepAlive() {

	for {
		select {
		case <-s.stopServe:
			log.Println("Stop Sending KeepAlive Messages")
			break
		default:
			s.client.Timestamp = time.Now().Unix()

			body, err := json.Marshal(s.client)
			if err != nil {
				log.Fatalf("json encoder: %s", err)
			}
			s.Publish("keepalives", body)
			time.Sleep(20 * time.Second)
		}
	}
}
