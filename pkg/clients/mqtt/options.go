package mqtt

import (
	"fmt"
	"time"

	eclipsemqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/pflag"
	"k8s.io/klog/v2"
)

type MQTTClientOptions struct {
	BrokerHost string // tcp://127.0.0.1:1883
	Qos        int
	KeepAlive  time.Duration

	ConnEstablishingRetry int

	// TODO AuthMode string

	// TODO
	IncomingTopic string
	ResponseTopic string
}

func NewMQTTClientOptions() *MQTTClientOptions {
	return &MQTTClientOptions{
		Qos:                   0,
		KeepAlive:             3600 * time.Second,
		ConnEstablishingRetry: 10,
	}
}

func (o *MQTTClientOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BrokerHost, "mqtt-broker-host", o.BrokerHost, "The host of MQTT broker, e.g. tcp://127.0.0.1:1883")
	flags.IntVar(&o.Qos, "mqtt-qos", o.Qos, "")
	flags.DurationVar(&o.KeepAlive, "mqtt-keep-alive", o.KeepAlive, "")
	flags.IntVar(&o.ConnEstablishingRetry, "mqtt-conn-establishing-retry", o.ConnEstablishingRetry, "")
}

// TODO return an unified client, like cloud event client
func (o *MQTTClientOptions) GetClient(agentID string) (eclipsemqtt.Client, error) {
	var client eclipsemqtt.Client
	var err error
	for i := 0; i <= o.ConnEstablishingRetry; i++ {
		client, err = o.connectToBroker(agentID, o.BrokerHost)
		if err != nil {
			if i >= o.ConnEstablishingRetry {
				return nil, fmt.Errorf("failed to connect to MQTT broker %s, %v", o.BrokerHost, err)
			}

			klog.Warningf("Unable to connect to MQTT broker, %s, retrying", o.BrokerHost)
			time.Sleep(10 * time.Second)
			continue
		}

		break
	}
	return client, nil
}

func (o *MQTTClientOptions) connectToBroker(agentID, broker string) (eclipsemqtt.Client, error) {
	opts := eclipsemqtt.NewClientOptions().
		AddBroker(broker).
		SetClientID(agentID).
		SetKeepAlive(o.KeepAlive).
		SetAutoReconnect(true)

	client := eclipsemqtt.NewClient(opts)
	token := client.Connect()
	if token.Wait() && token.Error() != nil {
		return client, token.Error()
	}

	return client, nil
}
