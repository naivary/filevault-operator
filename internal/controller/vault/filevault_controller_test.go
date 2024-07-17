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

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	vaultv1alpha1 "github.com/naivary/filevault-operator/api/vault/v1alpha1"
)

var _ = Describe("Filevault Controller", func() {
	Context("When reconciling a resource", func() {
		const resourceName = "test-resource"

		ctx := context.Background()

		nn := types.NamespacedName{
			Name:      resourceName,
			Namespace: "default",
		}
		filevault := &vaultv1alpha1.Filevault{}

		BeforeEach(func() {
			By("creating the custom resource for the Kind Filevault")
			err := k8sClient.Get(ctx, nn, filevault)
			if err != nil && errors.IsNotFound(err) {
				resource := &vaultv1alpha1.Filevault{
					ObjectMeta: metav1.ObjectMeta{
						Name:      nn.Name,
						Namespace: nn.Namespace,
					},
					Spec: vaultv1alpha1.FilevaultSpec{
						Host:          "localhost",
						ContainerPort: 8080,
						MountPath:     "/mnt/filevault",
						ClaimName:     "test-pvc",
					},
				}
				Expect(k8sClient.Create(ctx, resource)).To(Succeed())
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
		})


		AfterEach(func() {
			resource := &vaultv1alpha1.Filevault{}
			err := k8sClient.Get(ctx, nn, resource)
			Expect(err).NotTo(HaveOccurred())

			By("Cleanup the specific resource instance Filevault")
			Expect(k8sClient.Delete(ctx, resource)).To(Succeed())
		})
	})
})
