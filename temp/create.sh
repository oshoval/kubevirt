#!/bin/bash
for i in {501..1000}
do
   cp test_vm1.yaml "test_vm"${i}.yaml
   sed -i "s/test-vm1/test-vm${i}/" "test_vm"${i}.yaml
   #echo "Welcome $i times"
done
