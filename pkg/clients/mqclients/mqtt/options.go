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
}

// TODO return an unified client, like cloud event client
// func (o *MQTTClientOptions) GetClient(agentID string) (pahomqtt.Client, error) {
// 	var client pahomqtt.Client
// 	var err error
// 	for i := 0; i <= o.ConnEstablishingRetry; i++ {
// 		client, err = o.connectToBroker(agentID, o.BrokerHost)
// 		if err != nil {
// 			if i >= o.ConnEstablishingRetry {
// 				return nil, fmt.Errorf("failed to connect to MQTT broker %s, %v", o.BrokerHost, err)
// 			}

// 			klog.Warningf("Unable to connect to MQTT broker, %s, retrying", o.BrokerHost)
// 			time.Sleep(10 * time.Second)
// 			continue
// 		}

// 		break
// 	}
// 	return client, nil
// }

// func (o *MQTTClientOptions) connectToBroker(agentID, broker string) (pahomqtt.Client, error) {
// 	opts := pahomqtt.NewClientOptions().
// 		AddBroker(broker).
// 		SetClientID(agentID).
// 		SetKeepAlive(o.KeepAlive).
// 		SetAutoReconnect(true)

// 	client := pahomqtt.NewClient(opts)
// 	token := client.Connect()
// 	if token.Wait() && token.Error() != nil {
// 		return client, token.Error()
// 	}

// 	return client, nil
// }
