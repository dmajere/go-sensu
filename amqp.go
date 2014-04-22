package main

import (
	"crypto/tls"
	"github.com/streadway/amqp"
	"log"
)

func rabbitConnect(url string, config *tls.Config) (*amqp.Connection, *amqp.Channel) {

	conn, err := amqp.DialTLS(url, config)
	if err != nil {
		log.Fatalf("amqp.DialTLS: %v", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		log.Fatalf("conn.Channel: %v", err)
	}
	return conn, ch
}

func rabbitDeclareExchanges(ch *amqp.Channel) {

	err := ch.ExchangeDeclare("keepalives", "direct", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("exchange.declare keepalive: %v", err)
	}

	err = ch.ExchangeDeclare("results", "direct", false, false, false, false, nil)
	if err != nil {
		log.Fatalf("exchange.declare results: %v", err)
	}
}

func rabbitDeclareQueue(ch *amqp.Channel, sub []string) {

	queue, err := ch.QueueDeclare("", true, true, false, false, nil)
	if err != nil {
		log.Fatalf("queue.declare: %v", err)
	}

	for _, exchange := range sub {

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

func setupRabbit(s *sensu) *amqp.Connection {
	var conn *amqp.Connection
	conn, s.rabbitChan = rabbitConnect(s.rabbitAddr.String(), s.rabbitCfg)
	rabbitDeclareExchanges(s.rabbitChan)
	rabbitDeclareQueue(s.rabbitChan, s.client.Subscriptions)
	return conn
}
