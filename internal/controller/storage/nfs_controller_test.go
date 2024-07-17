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
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	storagev1alpha1 "github.com/naivary/filevault-operator/api/storage/v1alpha1"
)

var _ = Describe("NFS Controller", func() {
	Context("When reconciling a resource", func() {
		const (
			serviceName = "nfs"
			serverName  = "nfs-server"
			volumeName  = "filevault-default-pv"

			resourceName = "test-nfs-resource"
			claimName    = "test-claimname"
			capacity     = "10Gi"

			timeout  = 10 * time.Second
			duration = 10 * time.Second
			interval = 250 * time.Millisecond
		)

		ctx := context.Background()
		nfs := &storagev1alpha1.NFS{}
		nn := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}

		BeforeEach(func() {
			By("creating the custom resource for the Kind NFS")
			err := k8sClient.Get(ctx, nn, nfs)
			if err != nil && errors.IsNotFound(err) {
				resource := &storagev1alpha1.NFS{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
					Spec: storagev1alpha1.NFSSpec{
						Capacity:  capacity,
						ClaimName: claimName,
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
			}
		})

		AfterEach(func() {
			resource := &storagev1alpha1.NFS{}
			err := k8sClient.Get(ctx, nn, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance NFS")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})

		It("should successfully reconcile the resource", func() {
			By("Reconciling the created resource")
			reconciler := &NFSReconciler{
				Client: k8sClient,
				Scheme: k8sClient.Scheme(),
			}

			_, err := reconciler.Reconcile(ctx, reconcile.Request{
				NamespacedName: nn,
			})
			Expect(err).NotTo(HaveOccurred())

			Eventually(func() bool {
				err := k8sClient.Get(ctx, nn, nfs)
				return err == nil
			}, timeout, interval).Should(BeTrue())

			Expect(nfs.Status.ClaimName).Should(Equal(claimName))
			Expect(nfs.Status.ServiceName).Should(Equal(serviceName))
			Expect(nfs.Status.ServerName).Should(Equal(serverName))
			Expect(nfs.Status.VolumeName).Should(Equal(volumeName))
		})
	})
})
