package mqtt

import (
	"context"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/paho"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/klog/v2"

	workv1 "open-cluster-management.io/api/work/v1"
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
	sync.Mutex

	options     *MQTTClientOptions
	msgChan     chan *paho.Publish
	clusterName string

	client *paho.Client

	objGenerations map[types.UID]int64
}

func NewMQTTClient(options *MQTTClientOptions, clusterName string) *MQTTClient {
	return &MQTTClient{
		options:        options,
		msgChan:        make(chan *paho.Publish),
		clusterName:    clusterName,
		objGenerations: make(map[types.UID]int64),
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
		work, err := decoder.Decode(m.Payload)
		if err != nil {
			klog.Errorf("failed to decode payload %s, %v", string(m.Payload), err)
			continue
		}

		evt := c.generateEvent(work)
		if evt == nil {
			continue
		}

		receiver.Receive(*evt)
	}

	return nil
}

// TODO: watch.Deleted
func (c *MQTTClient) generateEvent(work *workv1.ManifestWork) *watch.Event {
	c.Lock()
	defer c.Unlock()

	currentGen := work.Generation
	lastGen, ok := c.objGenerations[work.UID]
	if !ok {
		// add to current map
		c.objGenerations[work.UID] = currentGen
		work.CreationTimestamp = metav1.Now()
		return &watch.Event{
			Type:   watch.Added,
			Object: work,
		}
	}

	if currentGen == lastGen {
		return nil
	}

	// update current map
	c.objGenerations[work.UID] = currentGen
	return &watch.Event{
		Type:   watch.Modified,
		Object: work,
	}
}
