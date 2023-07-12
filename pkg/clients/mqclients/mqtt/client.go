package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/eclipse/paho.golang/paho"

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
	clientIDPrefix := "hub"
	if len(c.clusterName) != 0 {
		clientIDPrefix = c.clusterName
	}
	subClient, err := c.getConnect(
		ctx,
		fmt.Sprintf("%s-sub-client", clientIDPrefix),
		paho.NewSingleHandlerRouter(func(m *paho.Publish) {
			c.msgChan <- m
		}),
	)
	if err != nil {
		return err
	}

	pubClient, err := c.getConnect(ctx, fmt.Sprintf("%s-pub-client", clientIDPrefix), nil)
	if err != nil {
		return err
	}

	c.subClient = subClient
	c.pubClient = pubClient
	return nil
}

func (c *MQTTClient) Resync(ctx context.Context) error {
	_, _, resourceType := splitTopic(c.options.ResyncRequestTopic)
	data := c.generateResyncRequest(resourceType)
	if data == nil {
		return nil
	}

	_, err := c.pubClient.Publish(ctx, &paho.Publish{
		Topic:   c.options.ResyncRequestTopic,
		QoS:     byte(c.options.PubQoS),
		Payload: data,
	})
	if err != nil {
		return err
	}

	klog.Infof("Send resync request [%s] %s", c.options.ResyncRequestTopic, string(data))
	return nil
}

func (c *MQTTClient) Publish(ctx context.Context, work *workv1.ManifestWork) error {
	clusterName := c.clusterName
	if len(clusterName) == 0 {
		clusterName = work.Namespace
	}
	topic := strings.Replace(c.options.PubTopic, "+", clusterName, -1)
	_, _, resourceType := splitTopic(topic)

	var payload []byte
	var err error
	switch {
	case resourceType == "manifests":
		payload, err = c.generateSpecPayload(work)
		if err != nil {
			return err
		}
	case resourceType == "manifestsstatus":
		payload, err = c.generateStatusPayload(work)
		if err != nil {
			return err
		}
	default:
		klog.Warningf("unsupported topic type %s, topic=%s", resourceType, topic)
		return nil
	}

	if payload == nil {
		return nil
	}

	if _, err := c.pubClient.Publish(ctx, &paho.Publish{
		Topic:   topic,
		QoS:     byte(c.options.PubQoS),
		Payload: payload,
	}); err != nil {
		return err
	}

	klog.Infof("Send message - topic: [%s] - payload:\n\t%s\n", topic, payload)
	return nil
}

func (c *MQTTClient) Subscribe(ctx context.Context, receiver watcher.Receiver) error {
	sa, err := c.subClient.Subscribe(ctx, &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			c.options.SubTopic:            {QoS: byte(c.options.SubQoS)},
			c.options.ResyncResponseTopic: {QoS: byte(c.options.SubQoS)},
		},
	})
	if err != nil {
		// TODO consider retry and resync
		return fmt.Errorf("failed to subscribe to %s, %v", c.options.SubTopic, err)
	}

	if sa.Reasons[0] != byte(c.options.SubQoS) {
		return fmt.Errorf("failed to subscribe to %s, reason=%d", c.options.SubTopic, sa.Reasons[0])
	}

	klog.Infof("Subscribing to %s, %s", c.options.SubTopic, c.options.ResyncResponseTopic)

	for m := range c.msgChan {
		klog.Infof("Receive message - topic: [%s] - payload:\n\t%s\n", m.Topic, m.Payload)
		topicType, clusterName, resourceType := splitTopic(m.Topic)
		if topicType == "resync" {
			if err := c.resyncManifestworks(ctx, resourceType, clusterName, m); err != nil {
				klog.Errorf("failed to resync payload %s, %v", string(m.Payload), err)
			}
			continue
		}

		var evt *watch.Event
		switch {
		case resourceType == "manifests":
			work, err := c.decoder.DecodeSpec(m.Payload)
			if err != nil {
				klog.Warningf("failed to decode payload %s, %v", string(m.Payload), err)
				continue
			}
			evt = c.generateSpecEvent(work)
		case resourceType == "manifestsstatus":
			work, err := c.decoder.DecodeStatus(m.Payload)
			if err != nil {
				klog.Warningf("failed to decode payload %s, %v", string(m.Payload), err)
				continue
			}
			evt = c.generateStatusEvent(work)
		default:
			klog.Warningf("unsupported topic type %s, topic=%s", resourceType, m.Topic)
			return nil
		}

		if evt != nil {
			receiver.Receive(*evt)
		}
	}

	return nil
}

func (c *MQTTClient) generateSpecPayload(work *workv1.ManifestWork) ([]byte, error) {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Warningf("failed to get the work from store %v", err)
		return nil, nil
	}

	if exists {
		lastWork, ok := lastObj.(*workv1.ManifestWork)
		if !ok {
			klog.Warningf("failed to convert store object, %v", lastObj)
			return nil, nil
		}

		if work.DeletionTimestamp.IsZero() &&
			equality.Semantic.DeepEqual(lastWork.Spec, work.Spec) {
			klog.Infof("manifestwork spec is not changed, no need to publish spec")
			return nil, nil
		}
	}

	payload, err := c.encoder.EncodeSpec(work)
	if err != nil {
		return nil, err
	}

	return payload, nil
}

func (c *MQTTClient) generateSpecEvent(work *workv1.ManifestWork) *watch.Event {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Errorf("failed to get the work from store %v", err)
		return nil
	}

	if !exists {
		klog.Infof("manifestwork %s/%s doesn't exist, adding create event", work.Namespace, work.Name)
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
		klog.Infof("manifestwork %s/%s is not changed, no need update event", work.Namespace, work.Name)
		return nil
	}

	// TODO need a uniform function to do this
	// set required fields back
	work.CreationTimestamp = lastWork.CreationTimestamp
	work.Labels = lastWork.Labels
	work.Annotations = lastWork.Annotations
	work.Finalizers = lastWork.Finalizers
	work.Status = lastWork.Status

	return &watch.Event{
		Type:   watch.Modified,
		Object: work,
	}
}

func (c *MQTTClient) generateStatusPayload(work *workv1.ManifestWork) ([]byte, error) {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Warningf("failed to get the work from store %v", err)
		return nil, nil
	}

	if !exists {
		// do nothing
		klog.Infof("manifestwork %s/%s is not found, do nothing", work.Namespace, work.Name)
		return nil, nil
	}

	lastWork, ok := lastObj.(*workv1.ManifestWork)
	if !ok {
		klog.Warningf("failed to convert store object, %v", lastObj)
		return nil, nil
	}

	lastHash := lastWork.Annotations["statushash"]
	currentHash := work.Annotations["statushash"]

	// TODO do we need the aggregated status??
	if currentHash == lastHash {
		klog.Infof("manifestwork %s/%s status is not changed, need not to publish status", work.Namespace, work.Name)
		return nil, nil
	}

	data, err := c.encoder.EncodeStatus(work)
	if err != nil {
		return nil, err
	}

	return data, nil
}

func (c *MQTTClient) generateStatusEvent(work *workv1.ManifestWork) *watch.Event {
	lastObj, exists, err := c.store.Get(work)
	if err != nil {
		klog.Warningf("failed to get the work from store %v", err)
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

	work.Labels = lastWork.Labels
	work.DeletionTimestamp = lastWork.DeletionTimestamp
	work.Spec = lastWork.Spec

	if meta.IsStatusConditionTrue(work.Status.Conditions, payload.ResourceConditionTypeDeleted) {
		work.Finalizers = []string{}
		klog.Infof("Deleting manifestwork %s/%s", work.Namespace, work.Name)
		return &watch.Event{
			Type:   watch.Deleted,
			Object: work,
		}
	}

	lastStatusHash := lastWork.Annotations["statushash"]
	currentStatusHash := work.Annotations["statushash"]

	// no status change
	if lastStatusHash == currentStatusHash {
		return nil
	}

	// restore annotations
	for k, v := range lastWork.Annotations {
		_, ok := work.Annotations[k]
		if !ok {
			work.Annotations[k] = v
		}
	}

	//TODO can we update status here directly??
	if len(work.Status.Conditions) > 0 {
		// restore old conditions
		for _, oldCondition := range lastWork.Status.Conditions {
			if !meta.IsStatusConditionPresentAndEqual(work.Status.Conditions, oldCondition.Type, oldCondition.Status) {
				meta.SetStatusCondition(&work.Status.Conditions, oldCondition)
			}
		}
	}

	// the work is handled by agent, we make sure the fainlizer here
	work.Finalizers = []string{controllers.ManifestWorkFinalizer}
	return &watch.Event{
		Type:   watch.Modified,
		Object: work,
	}
}

func (c *MQTTClient) getConnect(ctx context.Context, clientID string, router paho.Router) (*paho.Client, error) {
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

	client := paho.NewClient(paho.ClientConfig{Conn: conn})

	if router != nil {
		client.Router = router
	}

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

	klog.Infof("MQTT client %s is connected to %s\n", clientID, c.options.BrokerHost)
	return client, nil
}

func (c *MQTTClient) generateResyncRequest(resourceType string) []byte {
	if c.store == nil {
		return []byte("{}")
	}

	switch {
	case resourceType == "manifests":
		resourceVersions := map[string]string{}

		objs := c.store.List()
		for _, obj := range objs {
			work, ok := obj.(*workv1.ManifestWork)
			if !ok {
				continue
			}

			resourceVersions[string(work.UID)] = work.ResourceVersion
		}

		data, _ := json.Marshal(resourceVersions)
		return data
	case resourceType == "manifestsstatus":
		statusHashes := map[string]string{}
		objs := c.store.List()
		for _, obj := range objs {
			work, ok := obj.(*workv1.ManifestWork)
			if !ok {
				continue
			}

			statusHashes[string(work.UID)] = work.Annotations["statushash"]
		}

		data, _ := json.Marshal(statusHashes)
		return data
	default:
		klog.Warningf("unsupported resync type %s", resourceType)
	}
	return nil
}

func (c *MQTTClient) resyncManifestworks(ctx context.Context, resyncType, clusterName string, m *paho.Publish) error {
	//TODO need wait the cache is ready? how can we get a ready cache?
	if c.store == nil {
		return nil
	}

	switch {
	case resyncType == "manifests":
		c.resyncSpecs(ctx, clusterName, m.Payload)
	case resyncType == "manifestsstatus":
		c.resyncStatus(ctx, m.Payload)
	default:
		klog.Warningf("unsupported resync type %s", resyncType)
		return nil
	}

	return nil
}

func (c *MQTTClient) resyncSpecs(ctx context.Context, clusterName string, payload []byte) {
	if len(clusterName) == 0 {
		return
	}

	resourceVersions := map[string]string{}
	if err := json.Unmarshal(payload, &resourceVersions); err != nil {
		klog.Warningf("failed to get resource versions, %v", err)
		return
	}

	// TODO handle the manifestworks do not exist in the hub, but exist in the spoke
	works := c.store.List()
	for _, workObj := range works {
		work, ok := workObj.(*workv1.ManifestWork)
		if !ok {
			continue
		}

		if work.Namespace != clusterName {
			continue
		}

		resourceVersion, ok := resourceVersions[string(work.UID)]
		if !ok {
			// a manifestwork that does not exist in the spoke
			resourceVersion = "0"
		}

		lastResourceVersion, err := strconv.ParseInt(resourceVersion, 10, 64)
		if err != nil {
			continue
		}

		currentResourceVersion, err := strconv.ParseInt(work.ResourceVersion, 10, 64)
		if err != nil {
			continue
		}

		if currentResourceVersion > lastResourceVersion {
			topic := strings.Replace(c.options.PubTopic, "+", work.Namespace, -1)
			payload, err := c.encoder.EncodeSpec(work)
			if err != nil {
				klog.Warningf("failed to encode spec, %v", err)
				continue
			}

			if _, err := c.pubClient.Publish(ctx, &paho.Publish{
				Topic:   topic,
				QoS:     byte(c.options.PubQoS),
				Payload: payload,
			}); err != nil {
				klog.Warningf("failed to publish spec, %v", err)
				continue
			}

			klog.Infof("Send resync message [%s] %s", topic, payload)
		}
	}
}

func (c *MQTTClient) resyncStatus(ctx context.Context, payload []byte) {
	statusHashes := map[string]string{}
	if err := json.Unmarshal(payload, &statusHashes); err != nil {
		klog.Warningf("failed to get resource versions, %v", err)
		return
	}

	works := c.store.List()
	for _, workObj := range works {
		work, ok := workObj.(*workv1.ManifestWork)
		if !ok {
			continue
		}

		lastHash, ok := statusHashes[string(work.UID)]
		if !ok {
			// TODO consider how to delete the manifestwork
			// the work is not in this cluster, do nothing
			continue
		}

		currentHash, ok := work.Annotations["statushash"]
		if !ok {
			// current work does not have staus yet, do nothing
			continue
		}

		if currentHash == lastHash {
			// the status is not changed, do nothing
			continue
		}

		// publish the status
		payload, err := c.encoder.EncodeStatus(work)
		if err != nil {
			klog.Errorf("failed to encode status, %v", err)
			continue
		}

		if _, err := c.pubClient.Publish(ctx, &paho.Publish{
			Topic:   c.options.PubTopic,
			QoS:     byte(c.options.PubQoS),
			Payload: payload,
		}); err != nil {
			klog.Warningf("failed to publish status, %v", err)
			continue
		}

		klog.Infof("Send resync message [%s] %s", c.options.PubTopic, payload)
	}
}

func splitTopic(topic string) (topicType, clusterName, resourceType string) {
	topics := strings.Split(topic, "/")
	if len(topics) != 3 {
		return topicType, clusterName, resourceType
	}

	return topics[0], topics[1], topics[2]
}
