package mqtt

import (
	"github.com/spf13/pflag"
)

type MQTTClientOptions struct {
	BrokerHost            string // 127.0.0.1:1883
	Username              string
	Password              string
	KeepAlive             uint16
	ConnEstablishingRetry int
	IncomingTopic         string
	IncomingQoS           int
	ResponseTopic         string
	ResponseQoS           int
}

func NewMQTTClientOptions() *MQTTClientOptions {
	return &MQTTClientOptions{
		KeepAlive:             3600,
		ConnEstablishingRetry: 10,
		IncomingTopic:         "/v1/cluster1/+/content",
		IncomingQoS:           0,
		ResponseTopic:         "/v1/shard1/cluster1/status",
		ResponseQoS:           0,
	}
}

func (o *MQTTClientOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BrokerHost, "mqtt-broker-host", o.BrokerHost, "The host of MQTT broker, e.g. tcp://127.0.0.1:1883")
	flags.StringVar(&o.Username, "mqtt-username", o.Username, "username")
	flags.StringVar(&o.Password, "mqtt-password", o.Password, "password")
	flags.Uint16Var(&o.KeepAlive, "mqtt-keep-alive", o.KeepAlive, "")
	flags.IntVar(&o.ConnEstablishingRetry, "mqtt-conn-establishing-retry", o.ConnEstablishingRetry, "")
	flags.StringVar(&o.IncomingTopic, "mqtt-incoming-topic", o.IncomingTopic, "incoming topic")
	flags.IntVar(&o.IncomingQoS, "mqtt-incoming-qos", o.IncomingQoS, "incoming Qos")
	flags.StringVar(&o.ResponseTopic, "mqtt-response-topic", o.ResponseTopic, "response topic")
	flags.IntVar(&o.ResponseQoS, "mqtt-response-qos", o.ResponseQoS, "response Qos")
}
