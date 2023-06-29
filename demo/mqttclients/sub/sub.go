package sub

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"os"
	"os/signal"
	"syscall"

	"github.com/eclipse/paho.golang/paho"
	"github.com/spf13/cobra"

	workv1 "open-cluster-management.io/api/work/v1"
)

var shutdownSignals = []os.Signal{os.Interrupt, syscall.SIGTERM}
var onlyOneSignalHandler = make(chan struct{})
var shutdownHandler chan os.Signal

var username, password string
var broker = "127.0.0.1:1883"
var topic = "/v1/clusters/+/status"

type ManifestStatus struct {
	ResourceID        string            `json:"resourceID"`
	ResourceVersion   int64             `json:"resourceVersion"`
	ResourceCondition ResourceCondition `json:"resourceCondition"`
	ResourceStatus    ResourceStatus    `json:"resourceStatus"`
}

type ResourceCondition struct {
	Type    string `json:"type"`
	Status  string `json:"status"`
	Reason  string `json:"reason"`
	Message string `json:"message"`
}

type ResourceStatus struct {
	Values []workv1.FeedbackValue `json:"values"`
}

func NewSub() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "sub",
		Short: "Subscribe resource status from MQTT",
		RunE: func(cmd *cobra.Command, args []string) error {
			shutdownCtx, cancel := context.WithCancel(context.TODO())

			shutdownHandler := setupSignalHandler()
			go func() {
				defer cancel()
				<-shutdownHandler
			}()

			ctx, terminate := context.WithCancel(shutdownCtx)
			defer terminate()

			// start to subscribe topic
			listener(ctx, broker, topic, username, password)

			<-ctx.Done()
			return nil
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&broker, "broker", broker, "The MQTT broker address.")
	flags.StringVar(&username, "username", username, "The MQTT broker username.")
	flags.StringVar(&password, "password", password, "The MQTT broker password.")
	flags.StringVar(&topic, "topic", topic, "The resource status update topic.")
	return cmd
}

func listener(ctx context.Context, broker, rTopic, username, password string) {
	conn, err := net.Dial("tcp", broker)
	if err != nil {
		log.Fatalf("Failed to connect to %s: %s", broker, err)
	}

	c := paho.NewClient(paho.ClientConfig{
		Conn: conn,
	})

	c.Router = paho.NewSingleHandlerRouter(func(m *paho.Publish) {
		if m.Properties != nil && m.Properties.CorrelationData != nil && m.Properties.ResponseTopic != "" {
			// fmt.Printf("Received message with response topic %s and correl id %s\n%s",
			// 	m.Properties.ResponseTopic, string(m.Properties.CorrelationData))

			var status ManifestStatus
			if err := json.NewDecoder(bytes.NewReader(m.Payload)).Decode(&status); err != nil {
				fmt.Printf("Failed to decode request %s, %v\n", string(m.Payload), err)
			}

			fmt.Printf("Received resouce status update:\n")
			fmt.Printf("\t ResourceID=%s, ResourceVersion=%d\n", status.ResourceID, status.ResourceVersion)
			fmt.Printf("\t ResourceCondition: %v\n", status.ResourceCondition)
			// for _, val := range status.ResourceStatus.Values {
			// 	if val.Value.Integer != nil {
			// 		fmt.Printf("\t ResourceStatus: %s=%d\n", val.Name, *val.Value.Integer)
			// 	}
			// }

			if _, err := c.Publish(context.Background(), &paho.Publish{
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
	})

	cp := &paho.Connect{
		KeepAlive:  30,
		CleanStart: true,
		ClientID:   "maestrosimulator-sub",
		Username:   username,
		Password:   []byte(password),
	}

	if username != "" {
		cp.UsernameFlag = true
	}
	if password != "" {
		cp.PasswordFlag = true
	}

	ca, err := c.Connect(context.Background(), cp)
	if err != nil {
		log.Fatal(err)
	}
	if ca.ReasonCode != 0 {
		log.Fatalf("Failed to connect to %s : %d - %s", broker, ca.ReasonCode, ca.Properties.ReasonString)
	}

	fmt.Printf("Connected to MQTT broker %s to subscribe %s \n", broker, topic)

	_, err = c.Subscribe(context.Background(), &paho.Subscribe{
		Subscriptions: map[string]paho.SubscribeOptions{
			rTopic: {QoS: 0},
		},
	})
	if err != nil {
		log.Fatalf("Failed to subscribe: %s", err)
	}
}

func setupSignalHandler() <-chan struct{} {
	return setupSignalContext().Done()
}

func setupSignalContext() context.Context {
	close(onlyOneSignalHandler) // panics when called twice

	shutdownHandler = make(chan os.Signal, 2)

	ctx, cancel := context.WithCancel(context.Background())
	signal.Notify(shutdownHandler, shutdownSignals...)
	go func() {
		<-shutdownHandler
		cancel()
		<-shutdownHandler
		os.Exit(1) // second signal. Exit directly.
	}()

	return ctx
}
