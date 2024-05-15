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

package controller

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	vaultv1alpha1 "github.com/naivary/filevault-operator/api/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FilevaultReconciler reconciles a Filevault object
type FilevaultReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

func isDirExisting(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func int32ptr(i int32) *int32 {
	return &i
}

func stringptr(s string) *string {
	return &s
}

//+kubebuilder:rbac:groups=vault.filevault.com,resources=filevaults,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=vault.filevault.com,resources=filevaults/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=vault.filevault.com,resources=filevaults/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Filevault object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.16.3/pkg/reconcile
func (r *FilevaultReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)

	var filevault vaultv1alpha1.Filevault
	if err := r.Get(ctx, req.NamespacedName, &filevault); err != nil {
		l.Error(err, "couldn't fetch filevault resource")
		return ctrl.Result{}, err
	}

	rootPath := "/tmp/filevault"
	dir := filepath.Join(rootPath, filevault.ObjectMeta.Name)

	isExisting, err := isDirExisting(dir)
	if err != nil {
		l.Error(err, "can't check the status of the directory")
		return ctrl.Result{}, err
	}

	if !isExisting {
		mode := os.FileMode(0755)
		if err := os.MkdirAll(dir, mode); err != nil {
			return ctrl.Result{}, err
		}
	}

	// check if deployment exits
	deployName := fmt.Sprintf("filevault-deploy-%s", req.Name)
	dep := &appsv1.Deployment{}
	err = r.Get(ctx, types.NamespacedName{Namespace: req.Namespace, Name: deployName}, dep)
	if err == nil {
		// object exits and the assumption is everything else does too
		return ctrl.Result{RequeueAfter: 1 * time.Minute}, nil
	}

	volumeMode := corev1.PersistentVolumeFilesystem
	pv := r.newPV(&filevault, dir, &volumeMode)
	pvc := r.newPVC(&filevault, &volumeMode, pv)
	deploy := r.newDeployment(&filevault, pvc)

	objs := []client.Object{pv, pvc, deploy}
	for _, obj := range objs {
		if err := r.Create(ctx, obj); err != nil {
			l.Error(err, "cannot create the given object", "obj", obj)
			return ctrl.Result{}, err
		}
	}

	filevault.Status.Capacity = filevault.Spec.Capacity
	filevault.Status.PVCName = pvc.ObjectMeta.Name
	filevault.Status.PVName = pv.ObjectMeta.Name
	filevault.Status.DeploymentName = deploy.ObjectMeta.Name

	if err := r.Status().Update(ctx, &filevault); err != nil {
		l.Error(err, "coudln't update the status of the filevault")
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *FilevaultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vaultv1alpha1.Filevault{}).
		Complete(r)
}

func (r *FilevaultReconciler) newDeployment(f *vaultv1alpha1.Filevault, pvc *corev1.PersistentVolumeClaim) *appsv1.Deployment {
	name := fmt.Sprintf("filevault-deplo-%s", f.ObjectMeta.Name)
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.ObjectMeta.Namespace,
			Name:      name,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: int32ptr(3),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": fmt.Sprintf("filevault-pod-%s", f.ObjectMeta.Name),
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": fmt.Sprintf("filevault-pod-%s", f.ObjectMeta.Name),
					},
				},
				Spec: corev1.PodSpec{
					Containers: []corev1.Container{
						{
							Name:  "filevault",
							Image: "naivary/filevault:latest",
							VolumeMounts: []corev1.VolumeMount{
								{
									MountPath: "/tmp/filevault",
									Name:      "filevault-volume",
								},
							},
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "filevault-volume",
							VolumeSource: corev1.VolumeSource{
								PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{ClaimName: pvc.ObjectMeta.Name},
							},
						},
					},
				},
			},
		},
	}

}

func (r *FilevaultReconciler) newPVC(f *vaultv1alpha1.Filevault, volumeMode *corev1.PersistentVolumeMode, pv *corev1.PersistentVolume) *corev1.PersistentVolumeClaim {
	return &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: f.ObjectMeta.Namespace,
			Name:      fmt.Sprintf("filevault-pvc-%s", f.ObjectMeta.Name),
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			VolumeMode:  volumeMode,
			Resources: corev1.ResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(f.Spec.Capacity)},
			},
			StorageClassName: stringptr("local-storage"),
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"name": pv.ObjectMeta.Name,
				},
			},
		},
	}

}

func (r *FilevaultReconciler) newPV(f *vaultv1alpha1.Filevault, dir string, volumeMode *corev1.PersistentVolumeMode) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("filevault-pv-%s", f.ObjectMeta.Name),
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(f.Spec.Capacity)},
			VolumeMode:                    volumeMode,
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimDelete,
			StorageClassName:              "local-storage",
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				Local: &corev1.LocalVolumeSource{
					Path: dir,
				},
			},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"k8s-n1", "k8s-n2"},
								},
							},
						},
					},
				},
			},
		},
	}

}
