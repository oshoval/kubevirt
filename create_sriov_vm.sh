#!/bin/bash


oc create -f nad_sriov_wo_vlan.yaml
oc create -f vmi_sriov_vlan_wo_mac.yaml

