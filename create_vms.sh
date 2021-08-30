#!/bin/bash

oc create -f test_vmi.yaml
oc create -f cirros.yaml
oc create -f alpine.yaml
oc create -f bad_vm2.yaml
