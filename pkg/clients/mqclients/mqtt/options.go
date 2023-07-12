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

	// topics:
	// clusters/<cluster-id>/manifests
	// clusters/<cluster-id>/manifestsstatus
	// --------------------------------
	// resync/<cluster-id>/manifests
	// resync/clusters/status

	// hub:   clusters/+/manifests send the workload to a specified cluster
	// spoke: clusters/cluster1/status  send the workload status to a specified cluster
	PubTopic string
	PubQoS   int

	// hub:   clusters/+/manifestsstatus  receive the workload status from spoke
	// spoke: clusters/cluster1/manifests receive the workload from hub
	SubTopic string
	SubQoS   int

	// hub:   resync/clusters/manifestsstatus request to get workload status from all clusters
	// spoke: resync/cluster1/manifests       request to get workload from a specified cluster
	ResyncRequestTopic string

	// hub:   resync/+/manifests              response to send workload to a specified cluster
	// spoke: resync/clusters/manifestsstatus response to send workload status to hub
	ResyncResponseTopic string
}

func NewMQTTClientOptions() *MQTTClientOptions {
	return &MQTTClientOptions{
		KeepAlive:             3600,
		ConnEstablishingRetry: 10,
	}
}

func (o *MQTTClientOptions) AddFlags(flags *pflag.FlagSet) {
	flags.StringVar(&o.BrokerHost, "mqtt-broker-host", o.BrokerHost, "The host of MQTT broker, e.g. tcp://127.0.0.1:1883")
	flags.StringVar(&o.Username, "mqtt-username", o.Username, "username")
	flags.StringVar(&o.Password, "mqtt-password", o.Password, "password")
	flags.Uint16Var(&o.KeepAlive, "mqtt-keep-alive", o.KeepAlive, "")
	flags.IntVar(&o.ConnEstablishingRetry, "mqtt-conn-establishing-retry", o.ConnEstablishingRetry, "")
	flags.StringVar(&o.SubTopic, "mqtt-sub-topic", o.SubTopic, "sub topic")
	flags.IntVar(&o.SubQoS, "mqtt-sub-qos", o.SubQoS, "sub Qos")
	flags.StringVar(&o.PubTopic, "mqtt-pub-topic", o.PubTopic, "pub topic")
	flags.IntVar(&o.PubQoS, "mqtt-response-qos", o.PubQoS, "response Qos")
	flags.StringVar(&o.ResyncRequestTopic, "mqtt-resync-request-topic", o.ResyncRequestTopic, "resync request topic")
	flags.StringVar(&o.ResyncResponseTopic, "mqtt-resync-response-topic", o.ResyncResponseTopic, "resync response topic")
}
