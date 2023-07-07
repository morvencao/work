#!/bin/bash

cd $(dirname ${BASH_SOURCE})

set -e

hub=${HUB:-hub}
c1=${CLUSTER1:-cluster1}
c2=${CLUSTER2:-cluster2}

hubctx="kind-${hub}"
c1ctx="kind-${c1}"
c2ctx="kind-${c2}"

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
kind create cluster --name "${c2}"

echo "Initialize the ocm hub cluster\n"
clusteradm init --wait --context ${hubctx}
joincmd=$(clusteradm get token --context ${hubctx} | grep clusteradm)

echo "Join cluster1 to hub\n"
$(echo ${joincmd} --force-internal-endpoint-lookup --wait --context ${c1ctx} | sed "s/<cluster_name>/$c1/g")

echo "Join cluster2 to hub\n"
$(echo ${joincmd} --force-internal-endpoint-lookup --wait --context ${c2ctx} | sed "s/<cluster_name>/$c2/g")

echo "Accept join of cluster1,cluster2"
clusteradm accept --context ${hubctx} --clusters ${c1},${c2} --wait

kubectl get managedclusters --all-namespaces --context ${hubctx}

image=${IMAGE_NAME:-quay.io/skeeey/work:mqtt}

hub_container_node_ip=$(docker inspect -f '{{range.NetworkSettings.Networks}}{{.IPAddress}}{{end}}' hub-control-plane)

sed -i "s|172.18.0.2|${hub_container_node_ip}|g" ./deploy/agent/deployment.yaml

kubectl --context ${c1ctx} -n open-cluster-management scale deploy klusterlet --replicas 0
kubectl --context ${c1ctx} -n open-cluster-management-agent delete deploy klusterlet-work-agent --ignore-not-found

kubectl --context ${c2ctx} -n open-cluster-management scale deploy klusterlet --replicas 0
kubectl --context ${c2ctx} -n open-cluster-management-agent delete deploy klusterlet-work-agent --ignore-not-found

cd ./deploy/agent &&
  kustomize edit set image quay.io/open-cluster-management/work=${image} &&
  kubectl --context ${c1ctx} apply -k . &&
  sed -i "s|cluster1|cluster2|g" deployment.yaml &&
  kubectl --context ${c2ctx} apply -k . &&
  sed -i "s|cluster2|cluster1|g" deployment.yaml &&
  cd ../..

cd ./deploy/hub &&
  kustomize edit set image quay.io/open-cluster-management/work=${image} &&
  kubectl --context ${hubctx} apply -k . &&
  cd ../..
