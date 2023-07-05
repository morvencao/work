## Get Started

1. Set OCM dev env(hub, cluster1)

```shell
export IMAGE_NAME=quay.io/skeeey/work:mqtt
./setup.sh
```

2. Create a manifestworkreplicaset resource:

```shell
cat << EOF | kubectl --context kind-hub apply -f -
apiVersion: cluster.open-cluster-management.io/v1beta1
kind: Placement
metadata:
  name: placement1
  namespace: default
spec:
  numberOfClusters: 
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
  name: mwrset-cronjob
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
          group: batch
          name: sync-app-cronjob
          namespace: default
          resource: cronjobs
    deleteOption:
      propagationPolicy: Foreground
    workload:
      manifests:
        - apiVersion: batch/v1
          kind: CronJob
          metadata:
            name: sync-app-cronjob
            namespace: default
          spec:
            schedule: '* * 1 * *'
            jobTemplate:
              spec:
                template:
                  spec:
                    containers:
                    - name: hello
                      image: busybox:1.28
                      imagePullPolicy: IfNotPresent
                      command:
                        - /bin/sh
                        - -c
                        - date; echo Hello from the Kubernetes cluster
                    restartPolicy: OnFailure
  placementRefs:
    - name: placement1
EOF
```

3. Check manifestworkreplicaset and manifestwork:

```shell
$ kubectl --context kind-hub get mwrs
NAME             PLACEMENT    FOUND   MANIFESTWORKS   APPLIED
mwrset-cronjob   AsExpected   True    AsExpected      True
$ kubectl --context kind-hub get manifestwork -A
NAMESPACE   NAME             AGE
cluster1    mwrset-cronjob   26s
```
