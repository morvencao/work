package main

import (
	"context"
	"crypto/md5"
	"flag"
	"fmt"
	"log"
	"os"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/client-go/tools/clientcmd"

	clusterclientset "open-cluster-management.io/api/client/cluster/clientset/versioned"
	workclientset "open-cluster-management.io/api/client/work/clientset/versioned"
	clusterv1beta1 "open-cluster-management.io/api/cluster/v1beta1"
	workapiv1 "open-cluster-management.io/api/work/v1"
	workapiv1alpha1 "open-cluster-management.io/api/work/v1alpha1"
)

func newDeployment(name string, replicas int32) *appsv1.Deployment {
	deployment := &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Deployment",
			APIVersion: "apps/v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Namespace: "default",
			Name:      name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": "busybox",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": "busybox",
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:            "busybox",
							Image:           "busybox:1.28",
							ImagePullPolicy: corev1.PullIfNotPresent,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: 80,
								},
							},
							Command: []string{"sh", "-c", "sleep 3600"},
						},
					},
				},
			},
		},
	}

	return deployment
}

func newManifestWork(namespace, name string, objects ...runtime.Object) *workapiv1.ManifestWork {
	work := &workapiv1.ManifestWork{}

	work.Namespace = namespace
	if name != "" {
		work.Name = name
	} else {
		work.GenerateName = "work-"
	}

	work.Labels = map[string]string{
		"created-for-testing": "true",
	}

	var manifests []workapiv1.Manifest
	for _, object := range objects {
		manifest := workapiv1.Manifest{}
		manifest.Object = object
		manifests = append(manifests, manifest)
	}
	work.Spec.Workload.Manifests = manifests

	return work
}

func main() {
	ctx := context.TODO()
	namespace := metav1.NamespaceDefault

	mwrsName := flag.String("mwrsname", "", "name of manifestworkreplicasets")
	replicas := flag.Int("replicas", 1, "deployment replicas")
	delete := flag.Bool("delete", false, "delete all manifestworkreplicasets")
	flag.Parse()

	hubRestConfig, err := clientcmd.BuildConfigFromFlags("", os.Getenv("KUBECONFIG"))
	if err != nil {
		log.Fatal(err)
	}

	hubWorkClient, err := workclientset.NewForConfig(hubRestConfig)
	if err != nil {
		log.Fatal(err)
	}

	hubClusterClient, err := clusterclientset.NewForConfig(hubRestConfig)
	if err != nil {
		log.Fatal(err)
	}

	mwrsClient := hubWorkClient.WorkV1alpha1().ManifestWorkReplicaSets(namespace)
	placementClient := hubClusterClient.ClusterV1beta1().Placements(namespace)
	placementDecisionsClient := hubClusterClient.ClusterV1beta1().PlacementDecisions(namespace)

	if *delete {
		mwrss, err := mwrsClient.List(ctx, metav1.ListOptions{})
		if err != nil {
			log.Fatal(err)
		}

		for _, mwrs := range mwrss.Items {
			err = mwrsClient.Delete(ctx, mwrs.Name, metav1.DeleteOptions{})
			if err != nil {
				log.Fatal(err)
			}

			mwrs, err := mwrsClient.Get(ctx, mwrs.Name, metav1.GetOptions{})
			if err == nil {
				mwrs.Finalizers = []string{}
				_, err = mwrsClient.Update(ctx, mwrs, metav1.UpdateOptions{})
				if err != nil {
					log.Fatal(err)
				}
			}
		}

		return
	}

	placementRef := workapiv1alpha1.LocalPlacementReference{Name: "placement-test"}
	placement := &clusterv1beta1.Placement{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placementRef.Name,
			Namespace: namespace,
		},
	}
	placementDecision := &clusterv1beta1.PlacementDecision{
		ObjectMeta: metav1.ObjectMeta{
			Name:      placement.Name,
			Namespace: namespace,
			Labels:    map[string]string{clusterv1beta1.PlacementLabel: placement.Name},
		},
	}
	_, err = placementDecisionsClient.Get(ctx, placement.Name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = placementClient.Create(ctx, placement, metav1.CreateOptions{})
		if err != nil {
			log.Fatal(err)
		}
		placementDecision, err = placementDecisionsClient.Create(ctx, placementDecision, metav1.CreateOptions{})
		if err != nil {
			log.Fatal(err)
		}
		placementDecision.Status.Decisions = []clusterv1beta1.ClusterDecision{{ClusterName: "cluster1"}}
		_, err = placementDecisionsClient.UpdateStatus(ctx, placementDecision, metav1.UpdateOptions{})
		if err != nil {
			log.Fatal(err)
		}
	}
	if err != nil {
		log.Fatal(err)
	}

	name := "mwrset-" + rand.String(5)
	if *mwrsName != "" {
		name = *mwrsName
	}

	// make sure generated deployment name is not changed for the ManifestWorkReplicaSet
	mwrsNameHash := fmt.Sprintf("%x", md5.Sum([]byte(name)))
	deploy := newDeployment("busybox-"+mwrsNameHash, int32(*replicas))
	work := newManifestWork("cluster1", "work-"+mwrsNameHash, deploy)
	mwrs := &workapiv1alpha1.ManifestWorkReplicaSet{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: workapiv1alpha1.ManifestWorkReplicaSetSpec{
			ManifestWorkTemplate: work.Spec,
			PlacementRefs:        []workapiv1alpha1.LocalPlacementReference{placementRef},
		},
	}

	existing, err := mwrsClient.Get(ctx, name, metav1.GetOptions{})
	if errors.IsNotFound(err) {
		_, err = mwrsClient.Create(ctx, mwrs, metav1.CreateOptions{})
		if err != nil {
			log.Fatal(err)
		}
		return
	}

	if err != nil {
		log.Fatal(err)
	}

	existing.Spec = mwrs.Spec
	_, err = mwrsClient.Update(ctx, existing, metav1.UpdateOptions{})
	if err != nil {
		log.Fatal(err)
	}
}
