#!/bin/bash

cd $(dirname ${BASH_SOURCE})

set -e

hub=${HUB:-hub}
c1=${CLUSTER1:-cluster1}

hubctx="kind-${hub}"
c1ctx="kind-${c1}"

cat <<EOF | kind create cluster --name "${hub}" --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 31320
    hostPort: 31320
    listenAddress: "127.0.0.1"
    protocol: TCP
EOF
kind create cluster --name "${c1}"

echo "Initialize the ocm hub cluster\n"
clusteradm init --wait --context ${hubctx}
joincmd=$(clusteradm get token --context ${hubctx} | grep clusteradm)

echo "Join cluster1 to hub\n"
$(echo ${joincmd} --force-internal-endpoint-lookup --wait --context ${c1ctx} | sed "s/<cluster_name>/$c1/g")

echo "Accept join of cluster1"
clusteradm accept --context ${hubctx} --clusters ${c1} --wait

kubectl get managedclusters --all-namespaces --context ${hubctx}

image=${IMAGE_NAME:-quay.io/skeeey/work:mqtt}

hub_container_node_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' hub-control-plane)

sed -i "s|172.18.0.2|${hub_container_node_ip}|g" ./deploy/agent/deployment.yaml

kubectl --context ${c1ctx} -n open-cluster-management scale deploy klusterlet --replicas 0
kubectl --context ${c1ctx} -n open-cluster-management-agent delete deploy klusterlet-work-agent

cd ./deploy/agent &&
  kustomize edit set image quay.io/open-cluster-management/work=${image} &&
  kubectl --context ${c1ctx} apply -k . &&
  cd ../..

cd ./deploy/hub &&
  kustomize edit set image quay.io/open-cluster-management/work=${image} &&
  kubectl --context ${hubctx} apply -k . &&
  cd ../..
