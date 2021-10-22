package custom

import (
	"context"
	"fmt"
	"strings"
	"time"

	coreV1 "k8s.io/api/core/v1"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
	appInformersV1 "k8s.io/client-go/informers/apps/v1"
	"k8s.io/client-go/kubernetes"
	appListersV1 "k8s.io/client-go/listers/apps/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
)

const SecretDeployer string = "secret-deployer"
const DefaultSecretPath string = "/etc/" + SecretDeployer + "-data/"

// Mandatory field
const DeploymentLabelSecretName string = "secret-name"

// Optional field
const DeploymentLabelSecretKeys string = "secret-keys"

// Secret Keys separator
const SecretKeysSeparator string = "."

type controller struct {
	clientset      kubernetes.Interface
	depLister      appListersV1.DeploymentLister
	workQueue      workqueue.RateLimitingInterface
	depCacheSynced cache.InformerSynced
}

func InitController(clientSet kubernetes.Interface, depInformer appInformersV1.DeploymentInformer) *controller {
	newController := &controller{
		clientset:      clientSet,
		depLister:      depInformer.Lister(),
		depCacheSynced: depInformer.Informer().HasSynced,
		workQueue:      workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), SecretDeployer),
	}
	depInformer.Informer().AddEventHandler(
		cache.ResourceEventHandlerFuncs{
			AddFunc: newController.handleAdd,
		},
	)
	return newController
}

func (c *controller) checkDeployments(ns, name string) error {
	// Get Deployment
	ctx := context.Background()
	deployment, err := c.depLister.Deployments(ns).Get(name)
	if err != nil {
		fmt.Println("Error in c.depLister.Deployments(ns).Get(): ", err.Error())
	}
	// Check the deployment labels
	depLabels := deployment.Labels
	if labelFilter, labelFilterOk := depLabels["app"]; labelFilterOk && labelFilter == SecretDeployer {
		if secretName, secretNameFilterOk := depLabels[DeploymentLabelSecretName]; secretNameFilterOk {
			// Get the secret from the api-server
			secret, err := c.clientset.CoreV1().Secrets(ns).Get(ctx, secretName, metaV1.GetOptions{})
			if err != nil {
				fmt.Println("Error in c.clientSet.CoreV1().Secrets(ns).Get(): ", err.Error())
				return err
			}
			// Mount secret as a volume
			err = c.mountSecretInDep(ctx, ns, name, *secret, depLabels)
			if err != nil {
				fmt.Println("Error in c.mountSecretInDep(): ", err.Error())
				return err
			}
		} else {
			fmt.Println("Secret name not found in the deployment metadata.labels - Skipping secret mount....")
		}
	}
	return nil
}

func (c *controller) mountSecretInDep(
	// Fetch the latest version of deployment and modifying it
	ctx context.Context, ns, name string, secret coreV1.Secret, depLabels map[string]string) error {
	deployment, err := c.clientset.AppsV1().Deployments(ns).Get(ctx, name, metaV1.GetOptions{})
	if err != nil {
		fmt.Println("Error in c.clientSet.AppsV1().Deployments(ns).Get(): ", err.Error())
	}
	secretVolume := coreV1.Volume{
		Name: secret.Name + "-secret-volume",
		VolumeSource: coreV1.VolumeSource{
			Secret: &coreV1.SecretVolumeSource{
				SecretName: secret.Name,
			},
		},
	}
	containerVolumeMount := coreV1.VolumeMount{
		Name:      secret.Name + "-secret-volume",
		MountPath: DefaultSecretPath,
		ReadOnly:  true,
	}
	deployment.ObjectMeta = metaV1.ObjectMeta{
		Name:      name,
		Namespace: ns,
		Labels:    map[string]string{"app": SecretDeployer},
	}
	// Get and add secret keys from deployment labels
	if secretKeys, secretKeysOk := depLabels[DeploymentLabelSecretKeys]; secretKeysOk {
		secretKeysList := strings.Split(secretKeys, SecretKeysSeparator)
		for _, key := range secretKeysList {
			_, keyCheckDataOk := secret.Data[key]
			_, keyCheckStringDataOk := secret.StringData[key]
			if !(keyCheckDataOk || keyCheckStringDataOk) {
				fmt.Printf("Key %s not found in the mentioned secret %s - Skipping mount for key %s\n", key, secret.Name, key)
				continue
			}
			secretVolume.VolumeSource.Secret.Items = append(secretVolume.VolumeSource.Secret.Items, coreV1.KeyToPath{
				Key:  key,
				Path: key,
			})
		}
	}
	// Appending the secret in the Deployment
	deployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, secretVolume)
	for i := range deployment.Spec.Template.Spec.Containers {
		deployment.Spec.Template.Spec.Containers[i].VolumeMounts = append(
			deployment.Spec.Template.Spec.Containers[i].VolumeMounts, containerVolumeMount)
	}
	// Updating the deployment for the new secret
	_, err = c.clientset.AppsV1().Deployments(ns).Update(ctx, deployment, metaV1.UpdateOptions{})
	if err != nil {
		fmt.Println("Error in c.clientSet.AppsV1().Deployments(ns).Update(): ", err.Error())
		return err
	}
	fmt.Printf("Deployment %s has been updated with desired keys in secret %s\n", name, secret.Name)
	return nil
}

func (c *controller) handleAdd(obj interface{}) {
	c.workQueue.Add(obj)
}

func (c *controller) Run(stopCh <-chan struct{}) {
	fmt.Println("Starting Custom Controller....")
	if !cache.WaitForCacheSync(stopCh, c.depCacheSynced) {
		fmt.Println("Waiting for the cache to be synced....")
	}

	go wait.Until(c.worker, 1*time.Second, stopCh)

	<-stopCh
}

func (c *controller) worker() {
	for c.processItem() {
	}
}

func (c *controller) processItem() bool {
	item, shutdown := c.workQueue.Get()
	if shutdown {
		return false
	}

	defer c.workQueue.Forget(item)

	key, err := cache.MetaNamespaceKeyFunc(item)
	if err != nil {
		fmt.Println("Error in cache.MetaNamespaceKeyFunc(): ", err.Error())
	}

	ns, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		fmt.Println("Error in cache.SplitMetaNamespaceKey(): ", err.Error())
	}

	err = c.checkDeployments(ns, name)
	if err != nil {
		fmt.Println("Error in c.checkDeployments(): ", err.Error())
		return false
	}
	return true
}
