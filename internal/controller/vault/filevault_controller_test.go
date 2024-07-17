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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vaultv1alpha1 "github.com/naivary/filevault-operator/api/vault/v1alpha1"
	"github.com/naivary/filevault-operator/util/typeutil"
)

func newTestPV(ns, name string, capacity string) corev1.PersistentVolume {
	return corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.PersistentVolumeSpec{
			Capacity: corev1.ResourceList{
				corev1.ResourceStorage: resource.MustParse(capacity),
			},
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteMany},
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				Local: &corev1.LocalVolumeSource{
					Path:   "/mnt/fs/k8s/test",
					FSType: typeutil.Ptr("ext4"),
				},
			},
			NodeAffinity: &corev1.VolumeNodeAffinity{
				Required: &corev1.NodeSelector{
					NodeSelectorTerms: []corev1.NodeSelectorTerm{
						corev1.NodeSelectorTerm{
							MatchExpressions: []corev1.NodeSelectorRequirement{
								{
									Key:      "kubernetes.io/hostname",
									Operator: corev1.NodeSelectorOpIn,
									Values:   []string{"k8s-n1"},
								},
							},
						},
					},
				},
			},
		},
	}
}

func newTestPVC(ns, name, pvName, capacity string) corev1.PersistentVolumeClaim {
	const storageClassName = ""

	accessModes := []corev1.PersistentVolumeAccessMode{
		corev1.ReadWriteMany,
	}
	requests := corev1.ResourceList{
		corev1.ResourceStorage: resource.MustParse(capacity),
	}
	return corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: ns,
		},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes:      accessModes,
			VolumeName:       pvName,
			StorageClassName: typeutil.Ptr(storageClassName),
			Resources:        corev1.VolumeResourceRequirements{Requests: requests},
		},
	}
}

var _ = Describe("Filevault Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			pvName       = "filevault-test-name"
			pvcName      = "filevault-pvc-test-name"
			capacity     = "1Gi"
			resourceName = "test-filevault-resource"

			timeout  = 10 * time.Second
			duration = 10 * time.Second
			interval = 250 * time.Millisecond
		)

		ctx := context.Background()
		nn := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		filevault := &vaultv1alpha1.Filevault{}
		pv := newTestPV(nn.Namespace, pvName, capacity)
		pvc := newTestPVC(nn.Namespace, pvcName, pvName, capacity)

		BeforeEach(func() {
			By("creating the custom resource for the Kind Filevault")
			err := k8sClient.Get(ctx, nn, filevault)
			if err != nil && errors.IsNotFound(err) {
				resc := &vaultv1alpha1.Filevault{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
					Spec: vaultv1alpha1.FilevaultSpec{
						Host:          "localhost",
						ContainerPort: 8080,
						MountPath:     "/mnt/filevault",
						ClaimName:     pvcName,
					},
				}
				Expect(k8sClient.Create(ctx, resc)).To(Succeed())
				Expect(k8sClient.Create(ctx, &pv)).To(Succeed())
				Expect(k8sClient.Create(ctx, &pvc)).To(Succeed())
			}
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			reconciler := &FilevaultReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nn,
			})
			Expect(err).NotTo(HaveOccurred())
			Eventually(func() bool {
				err := k8sClient.Get(ctx, nn, filevault)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(filevault.Status.ServerName).Should(Equal(nn.Name))
		})

		AfterEach(func() {
			resource := &vaultv1alpha1.Filevault{}
			err := k8sClient.Get(ctx, nn, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Filevault")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &pv)).To(Succeed())
			Expect(k8sClient.Delete(ctx, &pvc)).To(Succeed())
		})
	})
})
