package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/eclipse/paho.golang/paho"
	"github.com/eclipse/paho.golang/paho/extensions/rpc"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/meta"
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

type ReconcileStatus struct {
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type Request struct {
	SentTimestamp             int64           `json:"sentTimestamp"`
	ResourceID                string          `json:"resourceID"`
	ObservedMaestroGeneration int64           `json:"observedMaestroGeneration"`
	ObservedCreationTimestamp int64           `json:"observedCreationTimestamp"`
	ReconcileStatus           ReconcileStatus `json:"reconcileStatus"`
}

type MQTTClient struct {
	sync.Mutex

	options    *MQTTClientOptions
	subClient  *paho.Client
	pubHandler *rpc.Handler

	clusterName string

	msgChan  chan *paho.Publish
	gens     map[types.UID]int64
	requests map[types.UID]*Request
}

func NewMQTTClient(options *MQTTClientOptions, clusterName string) *MQTTClient {
	return &MQTTClient{
		options:     options,
		msgChan:     make(chan *paho.Publish),
		clusterName: clusterName,
		gens:        make(map[types.UID]int64),
		requests:    make(map[types.UID]*Request),
	}
}

func (c *MQTTClient) Connect(ctx context.Context) error {
	subClient, err := c.getConnect(ctx, true)
	if err != nil {
		return err
	}

	pubClient, err := c.getConnect(ctx, false)
	if err != nil {
		return err
	}

	rpcHanlder, err := rpc.NewHandler(ctx, pubClient)
	if err != nil {
		return fmt.Errorf("failed to create mqtt rpc handler, %v", err)
	}

	c.subClient = subClient
	c.pubHandler = rpcHanlder

	klog.Infof("Connected to %s\n", c.options.BrokerHost)
	return nil
}

func (c *MQTTClient) Publish(ctx context.Context, work *workv1.ManifestWork) error {
	playload := c.toStatusRequest(work)
	if playload == nil {
		return nil
	}

	resp, err := c.pubHandler.Request(ctx, &paho.Publish{
		Topic:   c.options.ResponseTopic,
		Payload: playload,
	})
	if err != nil {
		return err
	}

	// TODO do more checks
	klog.Infof("Received response: %s", string(resp.Payload))
	return nil
}

func (c *MQTTClient) Subscribe(ctx context.Context, decoder decoder.Decoder, receiver watcher.Receiver) error {
	sa, err := c.subClient.Subscribe(ctx, &paho.Subscribe{
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
	lastGen, ok := c.gens[work.UID]
	if !ok {
		// add to current map
		c.gens[work.UID] = currentGen
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
	c.gens[work.UID] = currentGen
	return &watch.Event{
		Type:   watch.Modified,
		Object: work,
	}
}

func (c *MQTTClient) getConnect(ctx context.Context, sub bool) (*paho.Client, error) {
	var err error
	var conn net.Conn

	for i := 0; i <= c.options.ConnEstablishingRetry; i++ {
		conn, err = net.Dial("tcp", c.options.BrokerHost)
		if err != nil {
			if i >= c.options.ConnEstablishingRetry {
				return nil, fmt.Errorf("failed to connect to MQTT broker %s, %v", c.options.BrokerHost, err)
			}

			klog.Warningf("Unable to connect to MQTT broker, %s, retrying", c.options.BrokerHost)
			time.Sleep(10 * time.Second)
			continue
		}

		break
	}

	clientID := fmt.Sprintf("%s-pub", c.clusterName)

	cc := paho.ClientConfig{
		Conn:   conn,
		Router: paho.NewSingleHandlerRouter(nil),
	}

	if sub {
		clientID = fmt.Sprintf("%s-sub", c.clusterName)

		cc.Router = paho.NewSingleHandlerRouter(func(m *paho.Publish) {
			c.msgChan <- m
		})
	}

	client := paho.NewClient(cc)

	client.SetDebugLogger(&DebugLogger{})
	client.SetErrorLogger(&ErrorLogger{})

	cp := &paho.Connect{
		KeepAlive:  c.options.KeepAlive,
		ClientID:   clientID,
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

	ca, err := client.Connect(ctx, cp)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MQTT broker %s, %v", c.options.BrokerHost, err)
	}
	if ca.ReasonCode != 0 {
		return nil, fmt.Errorf("failed to connect to MQTT broker %s, %d - %s",
			c.options.BrokerHost, ca.ReasonCode, ca.Properties.ReasonString)
	}

	return client, nil
}

func (c *MQTTClient) toStatusRequest(work *workv1.ManifestWork) []byte {
	reconcileStatus := ReconcileStatus{
		Status:  "False",
		Reason:  "Progressing",
		Message: fmt.Sprintf("The resouce is  in the progress to be applied on the cluster %s", c.clusterName),
	}

	if meta.IsStatusConditionTrue(work.Status.Conditions, workv1.WorkApplied) {
		reconcileStatus = ReconcileStatus{
			Status:  "True",
			Reason:  workv1.WorkApplied,
			Message: fmt.Sprintf("The resouce is applied on the cluster %s", c.clusterName),
		}
	}

	request := &Request{
		SentTimestamp:             time.Now().Unix(),
		ResourceID:                string(work.UID),
		ObservedMaestroGeneration: work.Generation,
		ObservedCreationTimestamp: work.CreationTimestamp.Unix(),
		ReconcileStatus:           reconcileStatus,
	}

	lastRequest, ok := c.requests[work.UID]
	if !ok {
		c.requests[work.UID] = request
		jsonData, _ := json.Marshal(request)
		return jsonData
	}

	if request.ObservedMaestroGeneration == lastRequest.ObservedMaestroGeneration &&
		equality.Semantic.DeepEqual(request.ReconcileStatus, lastRequest.ReconcileStatus) {
		return nil
	}

	c.requests[work.UID] = request
	jsonData, _ := json.Marshal(request)
	return jsonData
}
