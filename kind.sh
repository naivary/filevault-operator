#!/bin/bash

kind delete cluster

kind create cluster

curl -skSL https://raw.githubusercontent.com/kubernetes-csi/csi-driver-nfs/v4.7.0/deploy/install-driver.sh | bash -s v4.7.0 --
k apply -f config/crd/bases/storage.filevault.com_nfses.yaml
k apply -f config/crd/bases/vault.filevault.com_filevaults.yaml
