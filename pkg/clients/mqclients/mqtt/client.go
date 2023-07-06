package mqtt

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/paho"
	"github.com/eclipse/paho.golang/paho/extensions/rpc"

	"k8s.io/apimachinery/pkg/api/equality"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"

	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/clients/mqclients/decoder"
	"open-cluster-management.io/work/pkg/clients/mqclients/encoder"
	"open-cluster-management.io/work/pkg/clients/mqclients/payload"
	"open-cluster-management.io/work/pkg/clients/watcher"
	"open-cluster-management.io/work/pkg/spoke/controllers"
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
	encoder     encoder.Encoder
	decoder     decoder.Decoder
	subClient   *paho.Client
	pubClient   *paho.Client
	pubHandler  *rpc.Handler
	msgChan     chan *paho.Publish
	store       cache.Store
	clusterName string
}

func NewMQTTClient(options *MQTTClientOptions, clusterName string, restMapper meta.RESTMapper) *MQTTClient {
	return &MQTTClient{
		options:     options,
		encoder:     encoder.NewMQTTEncoder(clusterName),
		decoder:     decoder.NewMQTTDecoder(clusterName, restMapper),
		msgChan:     make(chan *paho.Publish),
		clusterName: clusterName,
	}
}

func (c *MQTTClient) SetStore(store cache.Store) {
	c.store = store
}

func (c *MQTTClient) GetByKey(namespace, name string) (*workv1.ManifestWork, error) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	obj, exists, err := c.store.GetByKey(key)
	if err != nil {
		return nil, err
	}
	if !exists {
		return nil, errors.NewNotFound(workv1.Resource("manifestwork"), name)
	}
	return obj.(*workv1.ManifestWork), nil
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
	c.pubClient = pubClient
	c.pubHandler = rpcHanlder

	klog.Infof("Connected to %s\n", c.options.BrokerHost)
	return nil
}

func (c *MQTTClient) PublishSpec(ctx context.Context, work *workv1.ManifestWork) error {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Errorf("failed to get the work from store %v", err)
		return nil
	}

	if exists {
		lastWork, ok := lastObj.(*workv1.ManifestWork)
		if !ok {
			klog.Errorf("failed to convert store object, %v", lastObj)
			return nil
		}

		if lastWork.ResourceVersion == work.ResourceVersion &&
			equality.Semantic.DeepEqual(lastWork.Spec, work.Spec) {
			klog.Infof("manifestwork is not changed, no need to publish spec")
			return nil
		}
	}

	clusterName := work.Namespace
	specPayloadJSON, err := c.encoder.EncodeSpec(work)
	if err != nil {
		return err
	}

	resp, err := c.pubHandler.Request(ctx, &paho.Publish{
		Topic:   fmt.Sprintf("/v1/%s/%s/content", clusterName, work.UID),
		Payload: specPayloadJSON,
	})
	if err != nil {
		return err
	}

	// TODO do more checks
	klog.Infof("Received response: %s", string(resp.Payload))
	return nil
}

func (c *MQTTClient) PublishStatus(ctx context.Context, work *workv1.ManifestWork) error {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Errorf("failed to get the work from store %v", err)
		return nil
	}

	if !exists {
		// do nothing
		klog.Infof("manifestwork %s/%s is not found, do nothing", work.Namespace, work.Name)
		return nil
	}

	lastWork, ok := lastObj.(*workv1.ManifestWork)
	if !ok {
		klog.Errorf("failed to convert store object, %v", lastObj)
		return nil
	}

	if lastWork.ResourceVersion == work.ResourceVersion {
		klog.Infof("manifestwork is not changed, no need to publish status")
		return nil
	}

	statusPayloadJSON, err := c.encoder.EncodeStatus(work)
	if err != nil {
		klog.Errorf("failed to encode status %v", err)
		return err
	}

	resp, err := c.pubHandler.Request(ctx, &paho.Publish{
		Topic:   c.options.ResponseTopic,
		Payload: statusPayloadJSON,
	})
	if err != nil {
		return err
	}

	// TODO do more checks
	klog.Infof("Received response: %s", string(resp.Payload))
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
		klog.Infof("payload from MQTT, topic=%s, payload=%s", m.Topic, string(m.Payload))
		isStatusUpdate := isStatusUpdateTopic(m.Topic)
		work, err := c.getManifestWork(m.Payload, isStatusUpdate)
		if err != nil {
			klog.Errorf("failed to decode payload %s, %v", string(m.Payload), err)
			continue
		}

		evt := c.generateEvent(work, isStatusUpdate)
		if evt != nil {
			klog.Infof("add event to informer %v", evt)
			receiver.Receive(*evt)
		}

		if _, err := c.pubClient.Publish(ctx, &paho.Publish{
			Properties: &paho.PublishProperties{
				CorrelationData: m.Properties.CorrelationData,
			},
			Topic: m.Properties.ResponseTopic,
			// just return 0 to indicate the update success
			Payload: []byte("{\"code\":0}"),
		}); err != nil {
			fmt.Printf("Failed to publish message: %s\n", err)
		}
	}

	return nil
}

func (c *MQTTClient) generateEvent(work *workv1.ManifestWork, isStatusUpdate bool) *watch.Event {
	if isStatusUpdate {
		return c.generateStatusEvent(work)
	}

	return c.generateSpecEvent(work)
}

func (c *MQTTClient) generateSpecEvent(work *workv1.ManifestWork) *watch.Event {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Errorf("failed to get the work from store %v", err)
		return nil
	}

	if !exists {
		klog.Infof("manifestwork %s/%s doesn't exist, adding create event to informer", work.Namespace, work.Name)
		work.CreationTimestamp = metav1.Now()
		return &watch.Event{
			Type:   watch.Added,
			Object: work,
		}
	}

	lastWork, ok := lastObj.(*workv1.ManifestWork)
	if !ok {
		klog.Errorf("failed to convert store object, %v", lastObj)
		return nil
	}

	if !work.DeletionTimestamp.IsZero() {
		work.Generation = lastWork.Generation + 1
		work.Spec = lastWork.Spec
	}

	if work.Generation == lastWork.Generation {
		klog.Infof("manifestwork is not changed, no need to send update event to informer")
		return nil
	}

	// set required fields back
	work.Finalizers = lastWork.Finalizers
	work.CreationTimestamp = lastWork.CreationTimestamp
	work.ResourceVersion = lastWork.ResourceVersion
	work.Status = lastWork.Status

	return &watch.Event{
		Type:   watch.Modified,
		Object: work,
	}
}

func (c *MQTTClient) generateStatusEvent(work *workv1.ManifestWork) *watch.Event {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Errorf("failed to get the work from store %v", err)
		return nil
	}

	if !exists {
		klog.Infof("manifestwork %s/%s is not found, do nothing", work.Namespace, work.Name)
		return nil
	}

	lastWork, ok := lastObj.(*workv1.ManifestWork)
	if !ok {
		klog.Infof("failed to convert store object, %v", lastObj)
		return nil
	}

	// if !lastWork.DeletionTimestamp.IsZero() {
	// 	klog.Infof("manifework is deleting %s", work.Name)

	// 	return nil
	// }

	work.ObjectMeta = lastWork.ObjectMeta
	work.Spec = lastWork.Spec
	if len(work.Status.Conditions) > 0 {
		// restore old conditions
		for _, oldCondition := range lastWork.Status.Conditions {
			if !meta.IsStatusConditionPresentAndEqual(work.Status.Conditions, oldCondition.Type, oldCondition.Status) {
				meta.SetStatusCondition(&work.Status.Conditions, oldCondition)
			}
		}
	}

	if meta.IsStatusConditionTrue(work.Status.Conditions, payload.ResourceConditionTypeDeleted) {
		work.Finalizers = []string{}
		return &watch.Event{
			Type:   watch.Deleted,
			Object: work,
		}
	} else {
		work.Finalizers = []string{controllers.ManifestWorkFinalizer}
		return &watch.Event{
			Type:   watch.Modified,
			Object: work,
		}
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

	clientID := "hub-pub"
	if c.clusterName != "" {
		clientID = fmt.Sprintf("%s-pub", c.clusterName)
	}

	cc := paho.ClientConfig{
		Conn:   conn,
		Router: paho.NewSingleHandlerRouter(nil),
	}

	if sub {
		clientID = "hub-sub"
		if c.clusterName != "" {
			clientID = fmt.Sprintf("%s-sub", c.clusterName)
		}

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

func (c *MQTTClient) getManifestWork(payload []byte, isStatusUpdate bool) (*workv1.ManifestWork, error) {
	if isStatusUpdate {
		return c.decoder.DecodeStatus(payload)
	}

	return c.decoder.DecodeSpec(payload)
}

func isStatusUpdateTopic(topic string) bool {
	if strings.Contains(topic, "status") {
		return true
	}

	return false
}
