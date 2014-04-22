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
	rabbitChan              *amqp.Channel
	rabbitConnectionClose   chan int
	rabbitConnectionRestart chan int

	publishChan chan *Message
	exitChan    chan int
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
		publishChan:             make(chan *Message),
		exitChan:                make(chan int),
	}

	s.client.Keepalive.Thresholds.Warning = 120
	s.client.Keepalive.Thresholds.Critical = 180

	return s
}

func (s *sensu) Start() {
	go s.setupRabbit()
	s.ConsumeAndServe()
	go s.standaloneSetup()
	go s.KeepAlive()
	go s.Publish()
}

func (s *sensu) Exit() {
	s.rabbitConnectionClose <- 1
}

func (s *sensu) setupRabbit() {
	var sync uint32
	conn := setupRabbit(s)
	for {
		select {
		case <-s.rabbitConnectionClose:
			conn.Close()
			break
		case <-s.rabbitConnectionRestart:
			if sync == 0 {
				atomic.StoreUint32(&sync, 1)
				conn = setupRabbit(s)
				time.Sleep(5 * time.Second)
				atomic.StoreUint32(&sync, 0)
			}
		default:
		}
	}
}

func (s *sensu) ConsumeAndServe() {
	c := s.Consume()
	go s.Serve(c)
}

func (s *sensu) Consume() <-chan amqp.Delivery {

	checks, err := s.rabbitChan.Consume("", s.client.Name, false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}
	return checks
}

func (s *sensu) Serve(checks <-chan amqp.Delivery) {
	for {
		select {
		case <-s.exitChan:
			break
		case msg, ok := <-checks:
			if !ok {
				s.rabbitConnectionRestart <- 1
			} else {
				go s.handleCheck(&msg)
			}
		default:
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

	result := &RemoteResult{
		Client: s.client.Name,
		Check:  check,
	}

	body, err := json.Marshal(result)
	if err != nil {
		log.Fatalf("json encoder: %s", err)
	}
	s.publishChan <- &Message{"results", body}
}

func (s *sensu) Publish() {

	for message := range s.publishChan {

		msg := amqp.Publishing{
			DeliveryMode: amqp.Persistent,
			Timestamp:    time.Now(),
			ContentType:  "text/plain",
			Body:         message.body,
		}
		err := s.rabbitChan.Publish(message.exchange, "", false, false, msg)
		if err != nil {
			log.Fatalf("channel.publish: %s", err)
		}
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

				s.publishChan <- &Message{"results", body}

				time.Sleep(time.Duration(check.Interval) * time.Second)
			}
		}()
	}
}

func (s *sensu) KeepAlive() {

	for {

		s.client.Timestamp = time.Now().Unix()

		body, err := json.Marshal(s.client)
		if err != nil {
			log.Fatalf("json encoder: %s", err)
		}

		s.publishChan <- &Message{"keepalives", body}
		time.Sleep(20 * time.Second)
	}
}
