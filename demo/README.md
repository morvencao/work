# Get Started

## Deploy hub and spoke

1. Set KinD clusters(hub, cluster1)

```shell
export IMAGE_NAME=quay.io/skeeey/work:mqtt
./setup.sh
```

2. (Optional)Check work-hub-manager deployment and logs

```shell
kubectl --context=kind-hub -n open-cluster-management-hub get deploy work-hub-controller
kubectl --context=kind-hub -n open-cluster-management-hub logs -f deploy/work-hub-controller
```

3. (Optional)Check work-agent deployment and logs:

```shell
kubectl --context=kind-cluster1 -n open-cluster-management-agent get deploy work-agent
kubectl --context=kind-cluster1 -n open-cluster-management-agent logs -f deploy/work-agent
```

## Create manifestworkreplicaset

1. Create a manifestworkreplicaset resource:

```shell
export KUBECONFIG=~/.kube/config
kubectl config use-context kind-hub
go run ./demo/resource/main.go -mwrsname=mwrset1
```

2. Verify the status of manifestworkreplicaset is updated:

```shell
$ kubectl --context kind-hub get mwrs
NAME             PLACEMENT    FOUND   MANIFESTWORKS   APPLIED
mwrset1          AsExpected   True    AsExpected      True
```

3. Verify deploy is created on spoke cluster:

```shell
$ kubectl --context kind-cluster1 get deploy
NAME                                       READY   UP-TO-DATE   AVAILABLE   AGE
busybox-481505d79d86c45f89a02473b6530932   0/1     1            0           29s
```

## Update manifestworkreplicaset

1. Add more replicas by updating the manifestworkreplicaset resource:

```shell
go run ./demo/resource/main.go -mwrsname=mwrset1 -replicas 2
```

2. Verify deploy is updated on spoke cluster:

```shell
$ kubectl --context kind-cluster1 get deploy
NAME                                       READY   UP-TO-DATE   AVAILABLE   AGE
busybox-481505d79d86c45f89a02473b6530932   0/2     2            0           66s
```

## Delete manifestworkreplicaset

1. Delete the manifestworkreplicaset resource:

```shell
go run ./demo/resource/main.go -mwrsname=mwrset1 -delete=true
```

2. Verify deploy is deleted on spoke cluster:

```shell
$ kubectl --context kind-cluster1 get deploy
No resources found in default namespace.
```
