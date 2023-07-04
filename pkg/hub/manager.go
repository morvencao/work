package hub

import (
	"context"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"
	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	clusterinformers "open-cluster-management.io/api/client/cluster/informers/externalversions"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	"open-cluster-management.io/work/pkg/clients"
	"open-cluster-management.io/work/pkg/clients/mqclients/mqtt"
	"open-cluster-management.io/work/pkg/hub/controllers/manifestworkreplicasetcontroller"
)

// WorkHubManagerOptions defines the flags for workload hub manager
type WorkHubManagerOptions struct {
	mqttOptions *mqtt.MQTTClientOptions
}

// NewWorkHubManagerOptions returns the flags with default value set
func NewWorkHubManagerOptions() *WorkHubManagerOptions {
	return &WorkHubManagerOptions{
		mqttOptions: mqtt.NewMQTTClientOptions(),
	}
}

// AddFlags register and binds the default flags
func (o *WorkHubManagerOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	o.mqttOptions.AddFlags(flags)
}

// RunWorkHubManager starts the controllers on hub.
func (o *WorkHubManagerOptions) RunWorkHubManager(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	hubWorkClientBuilder := clients.NewHubWorkClientBuilder().
		WithHubKubeconfig(controllerContext.KubeConfig).
		WithMQTTOptions(o.mqttOptions)
	hubWorkClientHolder, err := hubWorkClientBuilder.NewHubWorkClient(ctx)
	if err != nil {
		return err
	}

	hubWorkClientSet := hubWorkClientHolder.GetClientSet()

	hubWorkReplicaSetClient, err := workclientset.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	hubClusterClient, err := clusterclientset.NewForConfig(controllerContext.KubeConfig)
	if err != nil {
		return err
	}

	clusterInformerFactory := clusterinformers.NewSharedInformerFactory(hubClusterClient, 30*time.Minute)
	workInformerFactory := workinformers.NewSharedInformerFactory(hubWorkReplicaSetClient, 30*time.Minute)

	// we need a separated filtered manifestwork informers so we only watch the manifestworks that manifestworkreplicaset cares.
	// This could reduce a lot of memory consumptions
	manifestWorkInformerFactory := workinformers.NewSharedInformerFactoryWithOptions(hubWorkClientSet, 30*time.Minute, workinformers.WithTweakListOptions(
		func(listOptions *metav1.ListOptions) {
			selector := &metav1.LabelSelector{
				MatchExpressions: []metav1.LabelSelectorRequirement{
					{
						Key:      manifestworkreplicasetcontroller.ManifestWorkReplicaSetControllerNameLabelKey,
						Operator: metav1.LabelSelectorOpExists,
					},
				},
			}
			listOptions.LabelSelector = metav1.FormatLabelSelector(selector)
		},
	))

	workInformer := manifestWorkInformerFactory.Work().V1().ManifestWorks()
	hubWorkClientHolder.SetStore(workInformer.Informer().GetStore())

	manifestWorkReplicaSetController := manifestworkreplicasetcontroller.NewManifestWorkReplicaSetController(
		controllerContext.EventRecorder,
		hubWorkClientSet,
		hubWorkReplicaSetClient,
		workInformerFactory.Work().V1alpha1().ManifestWorkReplicaSets(),
		workInformer,
		clusterInformerFactory.Cluster().V1beta1().Placements(),
		clusterInformerFactory.Cluster().V1beta1().PlacementDecisions(),
	)

	go clusterInformerFactory.Start(ctx.Done())
	go workInformerFactory.Start(ctx.Done())
	go manifestWorkInformerFactory.Start(ctx.Done())
	go manifestWorkReplicaSetController.Run(ctx, 5)

	<-ctx.Done()
	return nil
}
