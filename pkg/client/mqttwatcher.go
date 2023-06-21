package client

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/watch"
	workv1 "open-cluster-management.io/api/work/v1"
	"open-cluster-management.io/work/pkg/spoke/controllers"
)

// Reporter hides the details of how an error is turned into a runtime.Object for
// reporting on a watch stream since this package may not import a higher level report.
type Reporter interface {
	// AsObject must convert err into a valid runtime.Object for the watch stream.
	AsObject(err error) runtime.Object
}

// StreamWatcher turns any stream for which you can write a Decoder interface
// into a watch.Interface.
type StreamWatcher struct {
	sync.Mutex
	reporter Reporter
	result   chan watch.Event
	done     chan struct{}

	mqttClient mqtt.Client
}

// NewStreamWatcher creates a StreamWatcher from the given decoder.
func NewStreamWatcher() *StreamWatcher {
	sw := &StreamWatcher{
		//source:   d,
		reporter: errors.NewClientErrorReporter(http.StatusInternalServerError, "sub", "ClientWatchDecoding"),
		// It's easy for a consumer to add buffering via an extra
		// goroutine/channel, but impossible for them to remove it,
		// so nonbuffered is better.
		result: make(chan watch.Event),
		// If the watcher is externally stopped there is no receiver anymore
		// and the send operations on the result channel, especially the
		// error reporting might block forever.
		// Therefore a dedicated stop channel is used to resolve this blocking.
		done: make(chan struct{}),
	}

	if sw.mqttClient == nil {
		var err error
		sw.mqttClient, err = ConnectToMQTT("127.0.0.1:1883")
		if err != nil {
			klog.Fatal(err)
		}
	}
	go sw.receive()
	return sw
}

// ResultChan implements Interface.
func (sw *StreamWatcher) ResultChan() <-chan watch.Event {
	return sw.result
}

// Stop implements Interface.
func (sw *StreamWatcher) Stop() {
	// Call Close() exactly once by locking and setting a flag.
	sw.Lock()
	defer sw.Unlock()
	// closing a closed channel always panics, therefore check before closing
	select {
	case <-sw.done:
	default:
		close(sw.done)
	}
}

// receive reads result from the decoder in a loop and sends down the result channel.
func (sw *StreamWatcher) receive() {
	defer utilruntime.HandleCrash()
	defer close(sw.result)
	defer sw.Stop()

	go func() {
		token := sw.mqttClient.Subscribe("/v1/cluster1/+/content", 0, func(client mqtt.Client, msg mqtt.Message) {
			topic := msg.Topic()
			payload := msg.Payload()
			klog.Infof("msg from MQQT topic=%s payload=%s", topic, string(payload))

			//name := strings.ReplaceAll(msg.Topic(), s.subTopic+"/", "")

			attrs := map[string]any{}
			if err := json.Unmarshal(payload, &attrs); err != nil {
				klog.Errorf("failed to unmarshal payload %s, %v", string(payload), err)
			}

			eventPayload, ok := attrs["payload"]
			if !ok {
				return
			}

			eventContent, ok := eventPayload.(map[string]any)
			if !ok {
				return
			}

			content, ok := eventContent["content"]
			if !ok {
				return
			}

			// TODO think about how to handle errors
			action, obj, err := sw.decode(content)
			if err != nil {
				sw.result <- watch.Event{
					Type:   watch.Error,
					Object: sw.reporter.AsObject(fmt.Errorf("unable to decode an event from the watch stream: %v", err)),
				}
				return
			}

			sw.result <- watch.Event{
				Type:   action,
				Object: obj,
			}
		})
		<-token.Done()
		//TODO: t error
	}()

	<-sw.done
}

// TODO:
// resourceVersion := meta.GetResourceVersion()
// watch.Added
// watch.Modified
// watch.Deleted
func (sw *StreamWatcher) decode(content any) (watch.EventType, runtime.Object, error) {
	jsonData, err := json.Marshal(content)
	if err != nil {
		return watch.Error, nil, err
	}

	manifests := []workv1.Manifest{}
	manifests = append(manifests, workv1.Manifest{
		RawExtension: runtime.RawExtension{Raw: jsonData},
	})

	work := &workv1.ManifestWork{
		TypeMeta: metav1.TypeMeta{},
		ObjectMeta: metav1.ObjectMeta{
			Name:       fmt.Sprintf("%s-%s", "cluster1", "test"),
			Namespace:  "cluster1",
			Finalizers: []string{controllers.ManifestWorkFinalizer},
			// Labels: map[string]string{
			// 	constants.KlusterletWorksLabel: "true",
			// },
		},
		Spec: workv1.ManifestWorkSpec{
			Workload: workv1.ManifestsTemplate{
				Manifests: manifests,
			},
			DeleteOption: &workv1.DeleteOption{
				PropagationPolicy: workv1.DeletePropagationPolicyTypeOrphan,
			},
		},
	}
	return watch.Added, work, nil
}

func ConnectToMQTT(mqttBrokerAddr string) (mqtt.Client, error) {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(mqttBrokerAddr)

	client := mqtt.NewClient(opts)
	t := client.Connect()
	<-t.Done()

	return client, t.Error()
}
