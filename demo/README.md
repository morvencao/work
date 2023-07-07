## Get Started

1. Set OCM dev env(hub, cluster1)

```shell
export IMAGE_NAME=quay.io/skeeey/work:mqtt
./setup.sh
```

2. Check work-hub-manager deployment and logs

```shell
kubectl --context=kind-hub -n open-cluster-management-hub get deploy work-hub-controller
kubectl --context=kind-hub -n open-cluster-management-hub logs -f deploy/work-hub-controller
```

3. Check work-agent deployment and logs:

```shell
kubectl --context=kind-cluster1 -n open-cluster-management-agent get deploy work-agent
kubectl --context=kind-cluster1 -n open-cluster-management-agent logs -f deploy/work-agent
```

4. Create a manifestworkreplicaset resource:

```shell
cat << EOF | kubectl --context kind-hub apply -f -
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: placement1
  namespace: default
spec:
  numberOfClusters: 2
  clusterSets:
    - default
---
apiVersion: cluster.open-cluster-management.io/v1beta2
kind: ManagedClusterSetBinding
metadata:
  name: default
  namespace: default
spec:
  clusterSet: default
---
apiVersion: work.open-cluster-management.io/v1alpha1
kind: ManifestWorkReplicaSet
metadata:
  name: mwrset1
  namespace: default
spec:
  manifestWorkTemplate:
    manifestConfigs:
      - feedbackRules:
          - jsonPaths:
              - name: lastSuccessfulTime
                path: .status.lastSuccessfulTime
            type: JSONPaths
        updateStrategy:
          type: Update
        resourceIdentifier:
          group: apps
          name: nginx
          namespace: default
          resource: deployments
    deleteOption:
      propagationPolicy: Foreground
    workload:
      manifests:
        - apiVersion: apps/v1
          kind: Deployment
          metadata:
            name: nginx
            namespace: default
            labels:
              app: nginx
          spec:
            replicas: 1
            selector:
              matchLabels:
                app: nginx
            template:
              metadata:
                labels:
                  app: nginx
              spec:
                containers:
                - name: nginx
                  image: nginx:1.14.2
                  imagePullPolicy: IfNotPresent
                  ports:
                  - containerPort: 80
  placementRefs:
    - name: placement1
EOF
```

5. Verify the status of manifestworkreplicaset is updated:

```shell
$ kubectl --context kind-hub get mwrs
NAME             PLACEMENT    FOUND   MANIFESTWORKS   APPLIED
mwrset1          AsExpected   True    AsExpected      True
```

6. Verify CronJob is created on managedcluster:

```shell
$ kubectl --context kind-cluster1 get deployment
NAME    READY   UP-TO-DATE   AVAILABLE   AGE
nginx   1/1     1            1           6m21s
```
