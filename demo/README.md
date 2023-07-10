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
kubectl --context=kind-cluster1 -n open-cluster-management-agent logs deploy/work-agent
kubectl --context=kind-cluster2 -n open-cluster-management-agent get deploy work-agent
kubectl --context=kind-cluster2 -n open-cluster-management-agent logs deploy/work-agent
```

## Create manifestworkreplicaset

1. Create a manifestworkreplicaset resource:

```shell
kubectl --context kind-hub apply -f resource/manifestworkreplicaset.yaml
```

2. Verify the status of manifestworkreplicaset is updated:

```shell
$ kubectl --context kind-hub get mwrs
NAME             PLACEMENT    FOUND   MANIFESTWORKS   APPLIED
mwrset1          AsExpected   True    AsExpected      True
```

3. Verify deployments are created on spoke clusters:

```shell
$ kubectl --context kind-cluster1 get deploy
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   0/1     1            0           17s
$ kubectl --context kind-cluster2 get deploy
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   0/1     1            0           20s
```

## Update manifestworkreplicaset

1. Add more replicas by updating the manifestworkreplicaset resource:

```shell
sed "s|replicas: 1|replicas: 2|g" resource/manifestworkreplicaset.yaml | kubectl --context kind-hub apply -f -
```

2. Verify deployments are updated on spoke clusters:

```shell
$ kubectl --context kind-cluster1 get deploy
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   0/2     2            0           1m34s
$ kubectl --context kind-cluster2 get deploy
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   0/2     2            0           1m39s
```

## Delete manifestworkreplicaset

1. Delete the manifestworkreplicaset resource:

```shell
kubectl --context kind-hub delete mwrs mwrset1
```

2. Verify deployments are deleted on spoke clusters:

```shell
$ kubectl --context kind-cluster1 get deploy
No resources found in default namespace.
$ kubectl --context kind-cluster2 get deploy
No resources found in default namespace.
```
