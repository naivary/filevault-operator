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

package vault

import (
	"context"
	"fmt"

	vaultv1alpha1 "github.com/naivary/filevault-operator/api/vault/v1alpha1"
	"github.com/naivary/filevault-operator/util/k8sutil"
	corev1 "k8s.io/api/core/v1"
	cond "k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"
)

// FilevaultReconciler reconciles a Filevault object
type FilevaultReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=vault.filevault.com,resources=filevaults,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=vault.filevault.com,resources=filevaults/status,verbs=get;update;patch
// +kubebuilder:rbac:groups=vault.filevault.com,resources=filevaults/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Filevault object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.18.2/pkg/reconcile
func (r *FilevaultReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	l := log.FromContext(ctx)
	fv := vaultv1alpha1.Filevault{}

	isFilevaultCRExisting, err := k8sutil.IsExisting(ctx, r.Client, req.NamespacedName, &fv)
	if err != nil {
		return ctrl.Result{}, err
	}
	if !isFilevaultCRExisting {
		l.Info("Custom Resource Filevault not found. Existing Reconcile")
		return ctrl.Result{}, nil
	}

	if err := r.Get(ctx, req.NamespacedName, &fv); err != nil {
		l.Error(err, "cannot get Filevault Custom Resource")
		return ctrl.Result{}, err
	}

	nn := k8sutil.NewNamespacedName(req)

	claim := corev1.PersistentVolumeClaim{}
	isClaimExisting, err := k8sutil.IsExisting(ctx, r.Client, nn(fv.Spec.ClaimName), &claim)
	if err != nil {
		l.Error(err, "cannot check if the defined ClaimName is existing")
		return ctrl.Result{}, err
	}
	if !isClaimExisting {
		l.Info("claim does not exist and will not be created by this controller")
		return ctrl.Result{}, fmt.Errorf("defined claimName does not exist. claimName: %s", fv.Spec.ClaimName)
	}

	server := corev1.Pod{}
	isServerExisting, err := k8sutil.IsExisting(ctx, r.Client, nn(fv.Status.ServerName), &server)
	if err != nil {
		l.Error(err, "cannot check if filevault server is already existing", "serverName", fv.Status.ServerName)
		return ctrl.Result{}, err
	}
	if !isServerExisting {
		l.Info("filevault server does not exist and will be created", "server_name", fv.Status.ServerName)
		server = r.newServer(req, fv.Spec)
		controllerutil.SetOwnerReference(&fv, &server, r.Scheme)
		if err := r.Create(ctx, &server); err != nil {
			return ctrl.Result{}, err
		}
	}

	// update status
	fv.Status.ServerName = req.Name
	fv.Status.ClaimName = fv.Spec.ClaimName
	cond.SetStatusCondition(&fv.Status.Conditions, vaultv1alpha1.NewFilevaultReadyCondition())
	err = r.Status().Update(ctx, &fv)
	return ctrl.Result{}, err
}

// SetupWithManager sets up the controller with the Manager.
func (r *FilevaultReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&vaultv1alpha1.Filevault{}).
		Complete(r)
}

func (r *FilevaultReconciler) newServer(req ctrl.Request, spec vaultv1alpha1.FilevaultSpec) corev1.Pod {
	const image = "naivary/filevault:latest"
	const mountPath = "/mnt/filevault"
	claimName := spec.ClaimName
	containerPort := int32(spec.ContainerPort)
	name := req.Name

	httpServerPort := corev1.ContainerPort{
		Name:          "http-server",
		ContainerPort: containerPort,
		Protocol:      corev1.ProtocolTCP,
	}

	claimMount := corev1.VolumeMount{
		MountPath: mountPath,
		Name:      claimName,
		ReadOnly:  false,
	}
	container := corev1.Container{
		Name:         name,
		Image:        image,
		Ports:        []corev1.ContainerPort{httpServerPort},
		VolumeMounts: []corev1.VolumeMount{claimMount},
		Env: []corev1.EnvVar{
			{
				Name:  "FILEVAULT_DIR",
				Value: mountPath,
			},
		},
	}
	claim := corev1.Volume{
		Name: claimName,
		VolumeSource: corev1.VolumeSource{
			PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
				ClaimName: claimName,
				ReadOnly:  false,
			},
		},
	}

	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: req.Namespace,
			Labels: map[string]string{
				"app": name,
			},
		},
		Spec: corev1.PodSpec{
			Containers: []corev1.Container{container},
			Volumes:    []corev1.Volume{claim},
		},
	}

}
