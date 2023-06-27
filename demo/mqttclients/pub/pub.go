package pub

import (
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"strings"

	"github.com/eclipse/paho.golang/paho"
	"github.com/ghodss/yaml"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var username, password, resouceFilePath string
var delete bool

var broker = "127.0.0.1:1883"
var resouceID = "b1e0ccaa-1d84-49dc-a98a-31a6fb2062cc"
var clusterName = "cluster1"

var updateStrategy = "Update"
var deletePolicy = "Foreground"

var generation int64 = 1

func NewPub() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pub",
		Short: "Publish a resource content to MQTT",
		Run: func(cmd *cobra.Command, args []string) {
			conn, err := net.Dial("tcp", broker)
			if err != nil {
				log.Fatalf("Failed to connect to %s, %v", broker, err)
			}

			c := paho.NewClient(paho.ClientConfig{
				Conn: conn,
			})

			cp := &paho.Connect{
				KeepAlive:  30,
				ClientID:   "maestrosimulator-pub",
				CleanStart: true,
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
				log.Fatalf("Failed to connect to %s, %v", broker, err)
			}
			if ca.ReasonCode != 0 {
				log.Fatalf("Failed to connect to %s, %d - %s", broker, ca.ReasonCode, ca.Properties.ReasonString)
			}

			jsonMsg := toMsg()

			topic := fmt.Sprintf("/v1/%s/%s/content", clusterName, resouceID)
			fmt.Printf("Publish resouce to MQTT broker %s\n", broker)
			fmt.Printf("Topic: %s\n", topic)
			fmt.Printf("Payload: %s\n", jsonMsg)

			if _, err = c.Publish(context.Background(), &paho.Publish{
				Topic:   topic,
				QoS:     byte(0),
				Payload: []byte(jsonMsg),
				Properties: &paho.PublishProperties{
					ContentType: "v1/json",
				},
			}); err != nil {
				log.Fatal("error sending message:", err)
			}

			fmt.Println("Message is sent")
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&broker, "broker", broker, "The MQTT broker address.")
	flags.StringVar(&username, "username", username, "The MQTT broker username.")
	flags.StringVar(&password, "password", password, "The MQTT broker password.")
	flags.StringVar(&clusterName, "cluster-name", clusterName, "The name of cluster.")
	flags.StringVar(&resouceID, "resouce-id", resouceID, "The ID of the resource")
	flags.Int64Var(&generation, "resouce-version", generation, "The version the resouce")
	flags.StringVar(&resouceFilePath, "resouce-file-path", resouceFilePath, "The file path of resource")
	flags.BoolVar(&delete, "delete", delete, "Delete the resouce")
	flags.StringVar(&updateStrategy, "update-strategy", updateStrategy, "Supported strategies: Update (default) or CreateOnly")
	flags.StringVar(&deletePolicy, "delete-policy", deletePolicy, "Supported delete policies: Foreground (default) or Orphan")

	return cmd
}

func toMsg() string {
	msgs := []string{fmt.Sprintf("\"resourceID\":\"%s\"", resouceID)}

	msgs = append(msgs, fmt.Sprintf("\"resourceVersion\":\"%d\"", generation))

	if len(updateStrategy) != 0 {
		msgs = append(msgs, fmt.Sprintf("\"updateStrategy\":\"%s\"", updateStrategy))
	}

	if len(deletePolicy) != 0 {
		msgs = append(msgs, fmt.Sprintf("\"deletePolicy\":\"%s\"", deletePolicy))
	}

	if delete {
		now := metav1.Now()
		msgs = append(msgs, fmt.Sprintf("\"deletionTimestamp\":\"%s\"", now.Format("2006-01-02T15:04:05Z")))
		return fmt.Sprintf("{%s}", strings.Join(msgs, ","))
	}

	content, err := os.ReadFile(resouceFilePath)
	if err != nil {
		log.Fatalf("Failed to read resource file from %s, %v", resouceFilePath, err)
	}

	manifest, err := yaml.YAMLToJSON(content)
	if err != nil {
		log.Fatalf("Failed to convert resource yaml to json, %v", err)
	}

	msgs = append(msgs, fmt.Sprintf("\"manifest\":%s", manifest))

	return fmt.Sprintf("{%s}", strings.Join(msgs, ","))
}
