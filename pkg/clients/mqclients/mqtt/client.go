package mqtt

import (
	"context"
	"fmt"
	"net"
	"time"

	paho "github.com/eclipse/paho.golang/paho"
	"k8s.io/klog/v2"
	"open-cluster-management.io/work/pkg/clients/decoder"
	"open-cluster-management.io/work/pkg/clients/watcher"
)

type DebugLogger struct{}

func (l *DebugLogger) Println(v ...interface{}) {
	klog.Infoln(v)
}

func (l *DebugLogger) Printf(format string, v ...interface{}) {
	klog.Infof(format, v...)
}

type ErrorLogger struct{}

func (l *ErrorLogger) Println(v ...interface{}) {
	klog.Errorln(v)
}

func (l *ErrorLogger) Printf(format string, v ...interface{}) {
	klog.Errorf(format, v...)
}

type MQTTClient struct {
	options     *MQTTClientOptions
	msgChan     chan *paho.Publish
	clusterName string

	client *paho.Client
}

func NewMQTTClient(options *MQTTClientOptions, clusterName string) *MQTTClient {
	return &MQTTClient{
		options:     options,
		msgChan:     make(chan *paho.Publish),
		clusterName: clusterName,
	}
}

func (c *MQTTClient) Connect(ctx context.Context) error {
	var err error
	var conn net.Conn

	for i := 0; i <= c.options.ConnEstablishingRetry; i++ {
		conn, err = net.Dial("tcp", c.options.BrokerHost)
		if err != nil {
			if i >= c.options.ConnEstablishingRetry {
				return fmt.Errorf("failed to connect to MQTT broker %s, %v", c.options.BrokerHost, err)
			}

			klog.Warningf("Unable to connect to MQTT broker, %s, retrying", c.options.BrokerHost)
			time.Sleep(10 * time.Second)
			continue
		}

		break
	}

	c.client = paho.NewClient(paho.ClientConfig{
		Router: paho.NewSingleHandlerRouter(func(m *paho.Publish) {
			c.msgChan <- m
		}),
		Conn: conn,
	})

	c.client.SetDebugLogger(&DebugLogger{})
	c.client.SetErrorLogger(&ErrorLogger{})

	cp := &paho.Connect{
		KeepAlive:  c.options.KeepAlive,
		ClientID:   c.clusterName,
		CleanStart: true,
	}

	if len(c.options.Username) != 0 {
		cp.Username = c.options.Username
		cp.UsernameFlag = true
	}
	if len(c.options.Password) != 0 {
		cp.Password = []byte(c.options.Password)
		cp.PasswordFlag = true
	}

	ca, err := c.client.Connect(ctx, cp)
	if err != nil {
		return fmt.Errorf("failed to connect to MQTT broker %s, %v", c.options.BrokerHost, err)
	}
	if ca.ReasonCode != 0 {
		return fmt.Errorf("failed to connect to MQTT broker %s, %d - %s",
			c.options.BrokerHost, ca.ReasonCode, ca.Properties.ReasonString)
	}

	fmt.Printf("Connected to %s\n", c.options.BrokerHost)

	return nil
}

func (c *MQTTClient) Publish(ctx context.Context) error {
	return nil
}

func (c *MQTTClient) Subscribe(ctx context.Context, decoder decoder.Decoder, receiver watcher.Receiver) error {
	sa, err := c.client.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			c.options.IncomingTopic: {QoS: byte(c.options.IncomingQoS)},
		},
	})
	if err != nil {
		return fmt.Errorf("failed to subscribe to %s, %v", c.options.IncomingTopic, err)
	}

	if sa.Reasons[0] != byte(c.options.IncomingQoS) {
		return fmt.Errorf("failed to subscribe to %s, reason=%d", c.options.IncomingTopic, sa.Reasons[0])
	}

	klog.Infof("Subscribed to %s", c.options.IncomingTopic)

	for m := range c.msgChan {
		klog.Infof("payload from MQQT %s", string(m.Payload))
		receiver.Receive(decoder.Decode(m.Payload))
	}

	return nil
}
