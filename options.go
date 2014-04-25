package main

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
	LogFile  string
}
