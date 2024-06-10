package k8sutil

import (
	"context"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func IsExisting(ctx context.Context, c client.Client, nn types.NamespacedName, obj client.Object) (bool, error) {
	err := c.Get(ctx, nn, obj)
	if err != nil && !apierrors.IsNotFound(err) {
		return false, err
	}
	if apierrors.IsNotFound(err) {
		return false, nil
	}
	return true, nil
}

func NewNamespacedName(req ctrl.Request) func(name string) types.NamespacedName {
	nn := types.NamespacedName{Namespace: req.Namespace}
	return func(name string) types.NamespacedName {
		nn.Name = name
		return nn
	}
}
