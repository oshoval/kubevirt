#!/bin/bash

echo "ALL PODS"
kubectl get pods -A
echo
kubectl get nodes

echo
echo "LOGS sriov-network-config-daemon"
echo

POD=$(kubectl get pods -n sriov-network-operator | grep sriov-network-config-daemon | awk '{print $1}')
kubectl logs -n sriov-network-operator $POD

