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

5. Verify the status of manifestworkreplicaset is updated:

```shell
$ kubectl --context kind-hub get mwrs
NAME             PLACEMENT    FOUND   MANIFESTWORKS   APPLIED
mwrset-cronjob   AsExpected   True    AsExpected      True
```

6. Verify CronJob is created on managedcluster:

```shell
$ kubectl --context kind-cluster1 get cronjob
NAME               SCHEDULE    SUSPEND   ACTIVE   LAST SCHEDULE   AGE
sync-app-cronjob   * * 1 * *   False     0        <none>          33s
```
