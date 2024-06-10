/*
Copyright 2024.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package storage

import (
	"context"
	"fmt"
	"path/filepath"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	storagev1alpha1 "github.com/naivary/filevault-operator/api/storage/v1alpha1"
	"github.com/naivary/filevault-operator/util/k8sutil"
	"github.com/naivary/filevault-operator/util/typeutil"
)

// NFSReconciler reconciles a NFS object
type NFSReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=storage.filevault.com,resources=nfs,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=storage.filevault.com,resources=nfs/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=storage.filevault.com,resources=nfs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the NFS object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *NFSReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	nfs := storagev1alpha1.NFS{}

	isNFSCustomResourceExisting, err := k8sutil.IsExisting(ctx, r.Client, req.NamespacedName, &nfs)
	if err != nil {
		l.Error(err, "cannot check if the custom resource NFS exists")
		return ctrl.Result{}, err
	}
	if !isNFSCustomResourceExisting {
		l.Info("Custom Resource NFS does not exist. Exiting reconcile loop")
		return ctrl.Result{}, nil
	}

	nn := k8sutil.NewNamespacedName(req)
	server := corev1.Pod{}

	isNFSServerExisting, err := k8sutil.IsExisting(ctx, r.Client, nn(nfs.Status.ServerName), &server)
	if err != nil {
		l.Error(err, "cannot check if the NFS server is existing", "server_name", nfs.Status.ServerName)
		return ctrl.Result{}, err
	}
	if !isNFSServerExisting {
		server = r.newServer(req)
		if err := r.Create(ctx, &server); err != nil {
			return ctrl.Result{}, err
		}
	}

	service := corev1.Service{}
	isServiceExisting, err := k8sutil.IsExisting(ctx, r.Client, nn(nfs.Status.ServiceName), &service)
	if err != nil {
		l.Error(err, "cannot check if service exists")
		return ctrl.Result{}, err
	}
	if !isServiceExisting {
		l.Info("nfs service does not exist. It will be created now.")
		service = r.newService(req)
		if err := r.Create(ctx, &service); err != nil {
			l.Error(err, "cannot create nfs service")
			return ctrl.Result{}, err
		}
	}

	pv := corev1.PersistentVolume{}
	isPVExisting, err := k8sutil.IsExisting(ctx, r.Client, nn(nfs.Status.VolumeName), &pv)
	if err != nil {
		l.Error(err, "cannot get persistent volume for filevault server")
		return ctrl.Result{}, err
	}
	if !isPVExisting {
		l.Info("persistent volume does not exist for the NFS Server. It will be created.")
		pv = r.newPV(req, nfs.Spec.Capacity)
		if err := r.Create(ctx, &pv); err != nil {
			l.Error(err, "cannot create persistent volume")
			return ctrl.Result{}, err
		}
	}

	pvc := corev1.PersistentVolumeClaim{}
	isPVCExisting, err := k8sutil.IsExisting(ctx, r.Client, nn(nfs.Status.ClaimName), &pvc)
	if err != nil {
		l.Error(err, "cannot check if PVC exits")
		return ctrl.Result{}, err
	}
	if !isPVCExisting {
		l.Info("persistent volume claim for nfs server does not exist. It will be created now.")
		pvc = r.newPVC(req, pv.Name, nfs.Spec.ClaimName, nfs.Spec.Capacity)
		if err := r.Create(ctx, &pvc); err != nil {
			l.Error(err, "cannot create PersistentVolumeClaim")
			return ctrl.Result{}, err
		}
	}

	// update status
	nfs.Status.ClaimName = pvc.Name
	nfs.Status.ServiceName = service.Name
	nfs.Status.ServerName = server.Name
	nfs.Status.VolumeName = pv.Name
	err = r.Status().Update(ctx, &nfs)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *NFSReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&storagev1alpha1.NFS{}).
		Complete(r)
}

func (r *NFSReconciler) newServer(req ctrl.Request) corev1.Pod {
	const name = "nfs-server"
	const image = "itsthenetwork/nfs-server-alpine:latest"
	const sharedDir = "/exports"
	const isPriviliged = true

	path := filepath.Join("/mnt/nfs/", req.Namespace)
	volumeName := fmt.Sprintf("nfs-server-%s-pv", req.Namespace)

	container := corev1.Container{
		Name:  name,
		Image: image,
		Env: []corev1.EnvVar{
			{
				Name:  "SHARED_DIRECTORY",
				Value: sharedDir,
			},
		},
		VolumeMounts: []corev1.VolumeMount{
			{
				Name:      volumeName,
				MountPath: sharedDir,
			},
		},
		SecurityContext: &corev1.SecurityContext{
			Privileged: typeutil.Ptr(isPriviliged),
		},
	}
	volume := corev1.Volume{
		Name: volumeName,
		VolumeSource: corev1.VolumeSource{
			HostPath: &corev1.HostPathVolumeSource{
				Path: path,
				Type: typeutil.Ptr(corev1.HostPathDirectoryOrCreate),
			},
		},
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"app": "nfs-server",
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
			Volumes:    []corev1.Volume{volume},
		},
	}
}

func (r *NFSReconciler) newService(req ctrl.Request) corev1.Service {
	const name = "nfs"
	tcp := corev1.ServicePort{
		Name:     "tcp-2049",
		Port:     2049,
		Protocol: corev1.ProtocolTCP,
	}
	udp := corev1.ServicePort{
		Name:     "udp-111",
		Port:     111,
		Protocol: corev1.ProtocolUDP,
	}
	return corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": "nfs-server",
			},
			Ports: []corev1.ServicePort{tcp, udp},
		},
	}
}

func (r *NFSReconciler) newPV(req ctrl.Request, capacity string) corev1.PersistentVolume {
	const csiDriver = "nfs.csi.k8s.io"
	var name = fmt.Sprintf("filevault-%s-pv", req.Namespace)
	var storageCapacity = resource.MustParse(capacity)
	var serverAddr = fmt.Sprintf("nfs.%s.svc.cluster.local", req.Namespace)
	var volumeHandle = fmt.Sprintf("%s/share##", serverAddr)

	accessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteMany,
	}
	mountOptions := []string{
		"nfsvers=4.1",
	}
	source := corev1.PersistentVolumeSource{
		CSI: &corev1.CSIPersistentVolumeSource{
			Driver:       csiDriver,
			VolumeHandle: volumeHandle,
			VolumeAttributes: map[string]string{
				"server": serverAddr,
				"share":  "/",
			},
		},
	}
	return corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
			Annotations: map[string]string{
				"pv.kubernetes.io/provisioned-by": csiDriver,
			},
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: storageCapacity,
			},
			AccessModes:                   accessModes,
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			MountOptions:                  mountOptions,
			PersistentVolumeSource:        source,
		},
	}
}

func (r *NFSReconciler) newPVC(req ctrl.Request, volumeName, claimName, capacity string) corev1.PersistentVolumeClaim {
	// PVC is namespace bound so the name can be equal for all
	const storageClassName = ""

	accessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteMany,
	}
	requests := corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse(capacity),
	}
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      claimName,
			Namespace: req.Namespace,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      accessModes,
			VolumeName:       volumeName,
			StorageClassName: typeutil.Ptr(storageClassName),
			Resources:        corev1.VolumeResourceRequirements{Requests: requests},
		},
	}
}
