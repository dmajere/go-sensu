package main

import (
        "fmt"
        "log"
        "time"
        "net/url"
        "os/exec"
        "crypto/tls"
        "crypto/x509"
        "encoding/json"
        "github.com/streadway/amqp"
)

type sensu struct {

	options *sensuOptions
	client  *sensuClient

	rabbitAddr *url.URL
	rabbitCfg  *tls.Config
	rabbitConn *amqp.Connection
	rabbitChan *amqp.Channel

	handlerChan chan sensuCheck
	exitChan    chan int

}

func Sensu(opt *sensuOptions) *sensu {

 
	rabbitCfg := new(tls.Config)
	rabbitCfg.RootCAs = x509.NewCertPool()
	cert, err := tls.X509KeyPair( ReadFile(opt.Rabbitmq.SSL.Cert_chain_file), ReadFile(opt.Rabbitmq.SSL.Private_key_file))
    if err != nil {
    	log.Fatal(err)
    }
    rabbitCfg.Certificates = append(rabbitCfg.Certificates, cert)
    rabbitCfg.InsecureSkipVerify=true

    rabbitAddr := url.URL {
    	        Scheme: "amqps",
                Host: fmt.Sprintf("%s:%d", opt.Rabbitmq.Host, opt.Rabbitmq.Port),
                Path: opt.Rabbitmq.Vhost,
                User: url.UserPassword(opt.Rabbitmq.User, opt.Rabbitmq.Password),
    }

    s := &sensu {

    	options: opt,
    	client:  &opt.Client,
    	rabbitAddr: &rabbitAddr,
    	rabbitCfg: rabbitCfg,
    	handlerChan: make(chan sensuCheck),
    	exitChan:   make(chan int),
    }

    return s
}

func (s *sensu) connectRabbit() {

	    conn, err := amqp.DialTLS( s.rabbitAddr.String(), s.rabbitCfg)
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

func (s *sensu) standaloneSetup() {

	for _, check := range s.options.Checks {
		go func() {
			for {
				s.handlerChan <- check
				time.Sleep(time.Duration(check.Interval) * time.Second)
			}
		}()
	}
}

func (s *sensu) keepAlive() {

        for {
                s.client.Timestamp = time.Now().Unix()
                body, err := json.Marshal(s.client)
                if err != nil {
                        log.Fatalf("json encoder: %s", err)
                }
                msg := amqp.Publishing{
                        DeliveryMode: amqp.Persistent,
                        Timestamp:    time.Now(),
                        ContentType:  "text/plain",
                        Body:         body,
                }
                err = s.rabbitChan.Publish("keepalives", "", false, false, msg)
                if err != nil {
                        log.Fatalf("channel.publish: %s", err)
                }
                time.Sleep(20 * time.Second)
        }
}

func (s *sensu) consume() {

	var check *sensuCheck
	checks, err := s.rabbitChan.Consume("", s.client.Name, false, false, false, false, nil)
	if err != nil {
		log.Fatal(err)
	}
	for {
		msg, ok := <-checks
        if !ok {
                log.Fatalf("source channel closed, see the reconnect example for handling this")
                break
        }
        json.Unmarshal(msg.Body, &check)
        s.handlerChan <- *check
    	}
}       





func (s *sensu) Serve() {
	for check := range s.handlerChan {
		
		go func() {
			    start := time.Now()
                out, err := exec.Command(check.Command).CombinedOutput()

                check.Executed = start.Unix()
                check.Duration = fmt.Sprintf( "%.3f", time.Since(start).Seconds())
                check.Status = 0
                check.Output = string(out)
                if err != nil {
                        check.Status = 1
                }

                result := &CheckResult{
                        Client: s.client.Name,
                        Check: &check,
                }

                fmt.Printf("Check %v\n", result)
			    fmt.Println(check.Command)
			//s.publish()
		}()
	}
}

func (s *sensu) Start() {

        s.connectRabbit()
        s.standaloneSetup()

        go s.keepAlive()
        go s.consume()

        go s.Serve()

}