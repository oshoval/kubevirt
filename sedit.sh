#!/bin/bash


sed -i 's#newName: nfvpe/sriov-device-plugin#newName: quay.io/oshoval/sriov-device-plugin#g' cluster-up/cluster/kind-k8s-sriov-1.17.0/sriov-components/manifests/kustomization.yaml

sed -i 's#newName: nfvpe/sriov-cni#newName: quay.io/oshoval/sriov-cni#g' cluster-up/cluster/kind-k8s-sriov-1.17.0/sriov-components/manifests/kustomization.yaml

