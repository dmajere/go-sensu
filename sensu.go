package main

import (
	"fmt"
	"log"
	"net/url"
	"time"

	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"github.com/streadway/amqp"
)

type sensu struct {
	options *sensuOptions
	client  *sensuClient

	rabbitAddr      *url.URL
	rabbitCfg       *tls.Config
	rabbitConn      *amqp.Connection
	rabbitChan      *amqp.Channel
	rabbitConnectionReady chan bool
	rabbitReconnect chan bool

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

		options:         opt,
		client:          &opt.Client,
		rabbitAddr:      &rabbitAddr,
		rabbitCfg:       rabbitCfg,
		rabbitReconnect: make(chan bool),
		rabbitConnectionReady: make(chan bool),
		publishChan:     make(chan *Message),
		exitChan:        make(chan int),
	}

	s.client.Keepalive.Thresholds.Warning = 120
	s.client.Keepalive.Thresholds.Critical =180
	
	return s
}

func (s *sensu) setupRabbit() {

	conn, err := amqp.DialTLS(s.rabbitAddr.String(), s.rabbitCfg)
	if err != nil {
		log.Fatalf("amqp.DialTLS: %v", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("conn.Channel: %v", err)
	}
	s.rabbitConn, s.rabbitChan = conn, ch

	err = ch.ExchangeDeclare("keepalives", "direct", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("exchange.declare keepalive: %v", err)
	}

	err = ch.ExchangeDeclare("results", "direct", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("exchange.declare results: %v", err)
	}

	queue, err := ch.QueueDeclare("", true, true, false, false, nil)
	if err != nil {
		log.Fatalf("queue.declare: %v", err)
	}

	for _, exchange := range s.client.Subscriptions {

		err = ch.ExchangeDeclare(exchange, "fanout", false, false, false, false, nil)
		if err != nil {
			log.Fatalf("exchange.declare: %v", err)
		}

		err = ch.QueueBind(queue.Name, "", exchange, false, nil)
		if err != nil {
			log.Fatalf("queue.bind: %v", err)
		}
	}
}

func (s *sensu) connectRabbit() {

	for {
		fmt.Println("Reconnect")
		s.setupRabbit()
		s.rabbitConnectionReady <- true
		<-s.rabbitReconnect	
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

func (s *sensu) Consume() {

	var check *sensuCheckRemote

	checks, err := s.rabbitChan.Consume("", s.client.Name, false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}

	for {

		msg, ok := <-checks
		if !ok {
			s.rabbitReconnect <- true
			<- s.rabbitConnectionReady
			continue
		}

		err = json.Unmarshal(msg.Body, &check)
		if err != nil {
			log.Fatal(err)
		}

		go func() {

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
		}()
	}
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



func (s *sensu) Start() {

	go s.connectRabbit()
	<- s.rabbitConnectionReady

	s.standaloneSetup()

	go s.KeepAlive()
	go s.Consume()
	go s.Publish()

}
