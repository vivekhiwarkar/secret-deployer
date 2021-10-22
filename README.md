# Secret Deployer

Mount a secret automatically just using labels in a new deployment

### Statement: 

We want to enable users to mount one secret on a workload (deployment) automatically if the workload has specific label or annotation set. Users should either be able to mount the entire secret as volume or just a key:value pair. You can make rest of the decision as you want to.

## Steps to run

Once the application is running, user can mount a secret as a VolumeMount automatically along with a deployment creation.

1. Just create a secret as shown in the example below;
``` {.sourceCode .bash}
apiVersion: v1
kind: Secret
type: Opaque
metadata:
  name: <test-secret> # secret name
stringData: # secret data
  name: Vivek Hiwarkar
  rank: Diamond
```

2. Next, create a normal deployment with the compulsory label 'app=secret-deployer' and 'secret-name=<actual-secret-name>' as shown below;
``` {.sourceCode .bash}
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: test-deployment
    app: secret-mountedeployer
    secret-name: <test-secret> # replace with secret name
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deployment
  template:
    metadata:
      labels:
        app: test-deployment
    spec:
      containers:
      - command:
        - ping
        - 8.8.8.8
        image: busybox:latest
        name: busybox
```

Resultant deployment will have all keys:values from secret test-secret present under path /etc/secret-deployer-data/ inside the pod container

### Mount a key:value secret

To mount specific keys:values use the optional label 'secret-keys=key1.key2.key3'. Refer following sample deployment YAML for usage;
``` {.sourceCode .bash}
apiVersion: apps/v1
kind: Deployment
metadata:
  labels:
    app: test-deployment
    app: secret-deployer
    secret-name: <test-secret>
    secret-keys: name.rank
  name: test-deployment
spec:
  replicas: 1
  selector:
    matchLabels:
      app: test-deployment
  template:
    metadata:
      labels:
        app: test-deployment
    spec:
      containers:
      - command:
        - ping
        - 8.8.8.8
        image: busybox:latest
        name: busybox
```
Resultant deployment will have mentioned keys:values from secret test-secret present under path /etc/secret-deployer-data/ inside the pod container

## How to install secret-deployer?

Requirment: A k8s cluster and a kubectl CLI configured to interact with the cluster.

Step 1: Download or clone this repository

Step 2: Run following command to install the application on your k8s-cluster

``` {.sourceCode .bash}
> kubectl apply -f secret-deployer/manifests/
```

Step 3: Wait for pods in secret-deployer namespace to reach 'Running' state

## How to test secret-deployer?

Create secret and deployment with mentioned labels

``` {.sourceCode .bash}
> kubectl apply -f secret-deployer/test
```

Run the following command to check secrets in the pod container for above deployment

``` {.sourceCode .bash}
> kubectl exec -it test-deployment-<hash-value-of-running-pod> -n default -- ls /etc/secret-deployer-data/
```
Required keys from the labels should be displayed as individual files. Contents to which will be the associated values.

Make sure the deployment is created in the same namespace as the secret. 