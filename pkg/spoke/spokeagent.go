package spoke

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"runtime/pprof"
	"time"

	"github.com/openshift/library-go/pkg/controller/controllercmd"
	"github.com/spf13/cobra"

	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/controller-runtime/pkg/client/apiutil"

	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	workinformers "open-cluster-management.io/api/client/work/informers/externalversions"
	ocmfeature "open-cluster-management.io/api/feature"
	"open-cluster-management.io/work/pkg/clients"
	"open-cluster-management.io/work/pkg/clients/mqclients/mqtt"
	"open-cluster-management.io/work/pkg/features"
	"open-cluster-management.io/work/pkg/spoke/auth"
	"open-cluster-management.io/work/pkg/spoke/controllers/appliedmanifestcontroller"
	"open-cluster-management.io/work/pkg/spoke/controllers/finalizercontroller"
	"open-cluster-management.io/work/pkg/spoke/controllers/manifestcontroller"
	"open-cluster-management.io/work/pkg/spoke/controllers/statuscontroller"
)

const (
	// If a controller queue size is too large (>500), the processing speed of the controller will drop significantly
	// with one worker, increasing the work numbers can imporve the processing speed.
	// We compared the two situations where the worker is set to 1 and 10, when the worker is 10, the resource
	// utilization of the kubeapi-server and work agent do not increase significantly.
	//
	// TODO expose a flag to set the worker for each controller
	appliedManifestWorkFinalizeControllerWorkers = 10
	manifestWorkFinalizeControllerWorkers        = 10
	availableStatusControllerWorkers             = 10
)

// WorkloadAgentOptions defines the flags for workload agent
type WorkloadAgentOptions struct {
	HubKubeconfigFile                      string
	SpokeKubeconfigFile                    string
	SpokeClusterName                       string
	AgentID                                string
	MemProfileFile                         string
	Burst                                  int
	StatusSyncInterval                     time.Duration
	AppliedManifestWorkEvictionGracePeriod time.Duration
	QPS                                    float32

	mqttOptions *mqtt.MQTTClientOptions
}

// NewWorkloadAgentOptions returns the flags with default value set
func NewWorkloadAgentOptions() *WorkloadAgentOptions {
	return &WorkloadAgentOptions{
		QPS:                                    50,
		Burst:                                  100,
		StatusSyncInterval:                     10 * time.Second,
		AppliedManifestWorkEvictionGracePeriod: 10 * time.Minute,
		mqttOptions:                            mqtt.NewMQTTClientOptions(),
	}
}

// AddFlags register and binds the default flags
func (o *WorkloadAgentOptions) AddFlags(cmd *cobra.Command) {
	flags := cmd.Flags()
	features.DefaultSpokeMutableFeatureGate.AddFlag(flags)
	// This command only supports reading from config
	flags.StringVar(&o.HubKubeconfigFile, "hub-kubeconfig", o.HubKubeconfigFile, "Location of kubeconfig file to connect to hub cluster.")
	flags.StringVar(&o.SpokeKubeconfigFile, "spoke-kubeconfig", o.SpokeKubeconfigFile,
		"Location of kubeconfig file to connect to spoke cluster. If this is not set, will use '--kubeconfig' to build client to connect to the managed cluster.")
	flags.StringVar(&o.SpokeClusterName, "spoke-cluster-name", o.SpokeClusterName, "Name of spoke cluster.")
	flags.StringVar(&o.AgentID, "agent-id", o.AgentID, "ID of the work agent to identify the work this agent should handle after restart/recovery.")
	flags.StringVar(&o.MemProfileFile, "memprofile-file", o.MemProfileFile, "Location of memory profile file.")
	flags.Float32Var(&o.QPS, "spoke-kube-api-qps", o.QPS, "QPS to use while talking with apiserver on spoke cluster.")
	flags.IntVar(&o.Burst, "spoke-kube-api-burst", o.Burst, "Burst to use while talking with apiserver on spoke cluster.")
	flags.DurationVar(&o.StatusSyncInterval, "status-sync-interval", o.StatusSyncInterval, "Interval to sync resource status to hub.")
	flags.DurationVar(&o.AppliedManifestWorkEvictionGracePeriod, "appliedmanifestwork-eviction-grace-period", o.AppliedManifestWorkEvictionGracePeriod, "Grace period for appliedmanifestwork eviction")

	o.mqttOptions.AddFlags(flags)
}

// RunWorkloadAgent starts the controllers on agent to process work from hub.
func (o *WorkloadAgentOptions) RunWorkloadAgent(ctx context.Context, controllerContext *controllercmd.ControllerContext) error {
	// load spoke client config and create spoke clients,
	// the work agent may not running in the spoke/managed cluster.
	spokeRestConfig, err := o.spokeKubeConfig(controllerContext)
	if err != nil {
		return err
	}

	spokeRestConfig.QPS = o.QPS
	spokeRestConfig.Burst = o.Burst

	spokeDynamicClient, err := dynamic.NewForConfig(spokeRestConfig)
	if err != nil {
		return err
	}

	spokeKubeClient, err := kubernetes.NewForConfig(spokeRestConfig)
	if err != nil {
		return err
	}

	spokeAPIExtensionClient, err := apiextensionsclient.NewForConfig(spokeRestConfig)
	if err != nil {
		return err
	}

	spokeWorkClient, err := workclientset.NewForConfig(spokeRestConfig)
	if err != nil {
		return err
	}

	restMapper, err := apiutil.NewDynamicRESTMapper(spokeRestConfig, apiutil.WithLazyDiscovery)
	if err != nil {
		return err
	}

	spokeWorkInformerFactory := workinformers.NewSharedInformerFactory(spokeWorkClient, 5*time.Minute)

	// build hub client and informer
	hubWorkClientBuilder := clients.NewHubWorkClientBuilder(o.SpokeClusterName, restMapper).
		WithHubKubeconfigFile(o.HubKubeconfigFile).
		WithMQTTOptions(o.mqttOptions)
	hubWorkClientHolder, err := hubWorkClientBuilder.NewHubWorkClient(ctx)
	if err != nil {
		return err
	}

	hubWorkClient := hubWorkClientHolder.GetClinet()
	hubWorkInformer := hubWorkClientHolder.GetInformer()
	hubWorkLister := hubWorkClientHolder.GetLister()
	hubhash := hubWorkClientHolder.GetHubHash()

	agentID := o.AgentID
	if len(agentID) == 0 {
		agentID = hubhash
	}

	validator := auth.NewFactory(
		spokeRestConfig,
		spokeKubeClient,
		hubWorkInformer,
		hubWorkLister,
		o.SpokeClusterName,
		controllerContext.EventRecorder,
		restMapper,
	).NewExecutorValidator(ctx, features.DefaultSpokeMutableFeatureGate.Enabled(ocmfeature.ExecutorValidatingCaches))

	manifestWorkController := manifestcontroller.NewManifestWorkController(
		controllerContext.EventRecorder,
		spokeDynamicClient,
		spokeKubeClient,
		spokeAPIExtensionClient,
		hubWorkClient,
		hubWorkInformer,
		hubWorkLister,
		spokeWorkClient.WorkV1().AppliedManifestWorks(),
		spokeWorkInformerFactory.Work().V1().AppliedManifestWorks(),
		restMapper,
		validator,
		o.SpokeClusterName, hubhash, agentID,
	)

	addFinalizerController := finalizercontroller.NewAddFinalizerController(
		controllerContext.EventRecorder,
		hubWorkClient,
		hubWorkInformer,
		hubWorkLister,
		o.SpokeClusterName,
	)

	appliedManifestWorkFinalizeController := finalizercontroller.NewAppliedManifestWorkFinalizeController(
		controllerContext.EventRecorder,
		spokeDynamicClient,
		spokeWorkClient.WorkV1().AppliedManifestWorks(),
		spokeWorkInformerFactory.Work().V1().AppliedManifestWorks(),
		agentID,
	)

	manifestWorkFinalizeController := finalizercontroller.NewManifestWorkFinalizeController(
		controllerContext.EventRecorder,
		hubWorkClient,
		hubWorkInformer,
		hubWorkLister,
		spokeWorkClient.WorkV1().AppliedManifestWorks(),
		spokeWorkInformerFactory.Work().V1().AppliedManifestWorks(),
		o.SpokeClusterName, hubhash,
	)

	unmanagedAppliedManifestWorkController := finalizercontroller.NewUnManagedAppliedWorkController(
		controllerContext.EventRecorder,
		hubWorkInformer,
		hubWorkLister,
		spokeWorkClient.WorkV1().AppliedManifestWorks(),
		spokeWorkInformerFactory.Work().V1().AppliedManifestWorks(),
		o.AppliedManifestWorkEvictionGracePeriod,
		o.SpokeClusterName, hubhash, agentID,
	)

	appliedManifestWorkController := appliedmanifestcontroller.NewAppliedManifestWorkController(
		controllerContext.EventRecorder,
		spokeDynamicClient,
		hubWorkClient,
		hubWorkInformer,
		hubWorkLister,
		spokeWorkClient.WorkV1().AppliedManifestWorks(),
		spokeWorkInformerFactory.Work().V1().AppliedManifestWorks(),
		o.SpokeClusterName,
		hubhash,
	)

	availableStatusController := statuscontroller.NewAvailableStatusController(
		controllerContext.EventRecorder,
		spokeDynamicClient,
		hubWorkClient,
		hubWorkInformer,
		hubWorkLister,
		o.StatusSyncInterval,
		o.SpokeClusterName,
	)

	go hubWorkInformer.Run(ctx.Done())
	go spokeWorkInformerFactory.Start(ctx.Done())

	go manifestWorkController.Run(ctx, 1)
	go addFinalizerController.Run(ctx, 1)
	go appliedManifestWorkFinalizeController.Run(ctx, appliedManifestWorkFinalizeControllerWorkers)
	go unmanagedAppliedManifestWorkController.Run(ctx, 1)
	go appliedManifestWorkController.Run(ctx, 1)
	go manifestWorkFinalizeController.Run(ctx, manifestWorkFinalizeControllerWorkers)
	go availableStatusController.Run(ctx, availableStatusControllerWorkers)

	if len(o.MemProfileFile) != 0 {
		// wait all controllers are started
		time.Sleep(30 * time.Second)

		f, err := os.Create(o.MemProfileFile)
		if err != nil {
			klog.Fatal("could not create memory profile: ", err)
		}
		defer f.Close()

		runtime.GC() // get up-to-date statistics

		if err := pprof.WriteHeapProfile(f); err != nil {
			klog.Fatalf("could not write memory profile: ", err)
		}
	}

	<-ctx.Done()

	return nil
}

// spokeKubeConfig builds kubeconfig for the spoke/managed cluster
func (o *WorkloadAgentOptions) spokeKubeConfig(controllerContext *controllercmd.ControllerContext) (*rest.Config, error) {
	if o.SpokeKubeconfigFile == "" {
		return controllerContext.KubeConfig, nil
	}

	spokeRestConfig, err := clientcmd.BuildConfigFromFlags("" /* leave masterurl as empty */, o.SpokeKubeconfigFile)
	if err != nil {
		return nil, fmt.Errorf("unable to load spoke kubeconfig from file %q: %w", o.SpokeKubeconfigFile, err)
	}
	return spokeRestConfig, nil
}
