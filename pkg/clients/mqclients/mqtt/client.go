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
	"open-cluster-management.io/work/pkg/clients/mqclients/decoder"
	"open-cluster-management.io/work/pkg/clients/mqclients/payload"
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

type workMeta struct {
	status            *payload.ManifestStatus
	gen               int64
	creationTimestamp metav1.Time
}

type MQTTClient struct {
	sync.Mutex
	options     *MQTTClientOptions
	decoder     decoder.Decoder
	subClient   *paho.Client
	pubHandler  *rpc.Handler
	msgChan     chan *paho.Publish
	workMetas   map[types.UID]*workMeta
	clusterName string
}

func NewMQTTClient(options *MQTTClientOptions, clusterName string) *MQTTClient {
	return &MQTTClient{
		options:     options,
		decoder:     &decoder.MQTTDecoder{ClusterName: clusterName},
		msgChan:     make(chan *paho.Publish),
		workMetas:   make(map[types.UID]*workMeta),
		clusterName: clusterName,
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
	workMeta, ok := c.workMetas[work.UID]
	if !ok {
		// this should be happened
		return nil
	}

	newStatus := toStatus(c.clusterName, work)
	lastStatus := workMeta.status
	if lastStatus != nil &&
		newStatus.ResourceVersion == lastStatus.ResourceVersion &&
		equality.Semantic.DeepEqual(newStatus.ResourceCondition, lastStatus.ResourceCondition) {
		return nil
	}

	workMeta.status = newStatus
	//c.workMetas[work.UID] = workMeta

	jsonPayload, _ := json.Marshal(newStatus)

	resp, err := c.pubHandler.Request(ctx, &paho.Publish{
		Topic:   c.options.ResponseTopic,
		Payload: jsonPayload,
	})
	if err != nil {
		return err
	}

	// TODO do more checks
	klog.Infof("Received response: %s", string(resp.Payload))

	if newStatus.ResourceCondition.Type == payload.ResourceConditionTypeDeleted {
		delete(c.workMetas, work.UID)
	}
	return nil
}

func (c *MQTTClient) Subscribe(ctx context.Context, receiver watcher.Receiver) error {
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
		work, err := c.decoder.Decode(m.Payload)
		if err != nil {
			klog.Errorf("failed to decode payload %s, %v", string(m.Payload), err)
			continue
		}

		evt := c.generateEvent(work)
		if evt == nil {
			continue
		}

		klog.Infof("add event to informer %v", evt)
		receiver.Receive(*evt)
	}

	return nil
}

func (c *MQTTClient) generateEvent(work *workv1.ManifestWork) *watch.Event {
	c.Lock()
	defer c.Unlock()

	currentGen := work.Generation
	lastWorkMeta, ok := c.workMetas[work.UID]
	if !ok {
		work.CreationTimestamp = metav1.Now()

		// add to current map
		lastWorkMeta := &workMeta{
			gen:               currentGen,
			creationTimestamp: work.CreationTimestamp,
		}
		c.workMetas[work.UID] = lastWorkMeta
		return &watch.Event{
			Type:   watch.Added,
			Object: work,
		}
	}

	// delete the workloads
	if !work.DeletionTimestamp.IsZero() {
		currentGen = currentGen + 1
	}

	if currentGen == lastWorkMeta.gen {
		return nil
	}

	// update current gen
	lastWorkMeta.gen = currentGen
	//c.gens[work.UID] = currentGen
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

// TODO aggregate the work status for reconcile status and this should be a common function
func toStatus(clusterName string, work *workv1.ManifestWork) *payload.ManifestStatus {
	if !work.DeletionTimestamp.IsZero() && len(work.Finalizers) == 0 {
		return &payload.ManifestStatus{
			ResourceID:      string(work.UID),
			ResourceVersion: work.Generation,
			ResourceCondition: payload.ResourceCondition{
				Type:    payload.ResourceConditionTypeDeleted,
				Status:  "True",
				Reason:  "Deleted",
				Message: fmt.Sprintf("The resouce is deleted from the cluster %s", clusterName),
			},
		}
	}

	status := &payload.ManifestStatus{
		ResourceID:      string(work.UID),
		ResourceVersion: work.Generation,
	}

	status.ResourceCondition = payload.ResourceCondition{
		Type:    payload.ResourceConditionTypeApplied,
		Status:  "False",
		Reason:  "Progressing",
		Message: fmt.Sprintf("The resouce is in the progress to be applied on the cluster %s", clusterName),
	}

	if meta.IsStatusConditionTrue(work.Status.Conditions, workv1.WorkApplied) {
		status.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeApplied,
			Status:  "True",
			Reason:  "Applied",
			Message: fmt.Sprintf("The resouce is applied on the cluster %s", clusterName),
		}
	}

	if meta.IsStatusConditionTrue(work.Status.Conditions, workv1.WorkAvailable) &&
		len(work.Status.ResourceStatus.Manifests) != 0 &&
		len(work.Status.ResourceStatus.Manifests[0].StatusFeedbacks.Values) != 0 {
		status.ResourceCondition = payload.ResourceCondition{
			Type:    payload.ResourceConditionTypeAvailable,
			Status:  "True",
			Reason:  "Available",
			Message: fmt.Sprintf("The resouce is available on the cluster %s", clusterName),
		}

		status.ResourceStatus = payload.ResourceStatus{
			Values: work.Status.ResourceStatus.Manifests[0].StatusFeedbacks.Values,
		}
	}

	return status
}
