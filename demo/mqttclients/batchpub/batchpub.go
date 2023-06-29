package batchpub

import (
	"fmt"
	"log"
	"os"
	"path"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/util/uuid"
	"open-cluster-management.io/work/demo/mqttclients/pub"
	"sigs.k8s.io/yaml"
)

func NewBPub() *cobra.Command {
	var broker = "127.0.0.1:1883"
	var username, password string
	var resourceDir string
	var groups int

	cmd := &cobra.Command{
		Use:   "bpub",
		Short: "Publish a group of resources content to MQTT",
		Run: func(cmd *cobra.Command, args []string) {
			for i := 1; i <= groups; i++ {
				fmt.Printf("#### pub group %d\n", i)
				entries, err := os.ReadDir(resourceDir)
				if err != nil {
					log.Fatal(err)
				}

				for _, entry := range entries {
					if entry.IsDir() {
						continue
					}

					name := entry.Name()

					if !strings.HasSuffix(name, ".yaml") {
						continue
					}

					resourceID := string(uuid.NewUUID())
					resourceFile := path.Join(resourceDir, name)

					fmt.Printf("%s\n", resourceFile)

					pub.Run(broker, username, password, "cluster1", resourceID, toMsg(resourceID, resourceFile, i))

					time.Sleep(2 * time.Second)
				}
				fmt.Printf("#### group %d is published\n", i)
			}
		},
	}

	flags := cmd.Flags()
	flags.StringVar(&broker, "broker", broker, "The MQTT broker address.")
	flags.StringVar(&username, "username", username, "The MQTT broker username.")
	flags.StringVar(&password, "password", password, "The MQTT broker password.")
	flags.StringVar(&resourceDir, "resource-dir", resourceDir, "The resource dir.")
	flags.IntVar(&groups, "groups", groups, "The groups.")

	return cmd
}

func toMsg(resourceID, resouceFilePath string, group int) string {
	msgs := []string{fmt.Sprintf("\"resourceID\":\"%s\"", resourceID)}

	msgs = append(msgs, "\"resourceVersion\":\"1\"")

	content, err := os.ReadFile(resouceFilePath)
	if err != nil {
		log.Fatalf("Failed to read resource file from %s, %v", resouceFilePath, err)
	}

	manifest, err := yaml.YAMLToJSON(content)
	if err != nil {
		log.Fatalf("Failed to convert resource yaml to json, %v", err)
	}

	newManifest := strings.ReplaceAll(string(manifest), "{suffix}", fmt.Sprintf("%d", group))

	msgs = append(msgs, fmt.Sprintf("\"manifest\":%s", newManifest))

	return fmt.Sprintf("{%s}", strings.Join(msgs, ","))
}
