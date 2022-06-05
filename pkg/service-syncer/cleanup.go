package service_syncer

import (
	"context"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"sigs.k8s.io/controller-runtime/pkg/client"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
)

func DelSyncedServices(ctx context.Context, k8sClient client.Client) error {
	svcList := &corev1.ServiceList{}
	if err := k8sClient.List(ctx, svcList, client.MatchingLabels{
		LabelManaged: LabelCrossClusterAssistant,
	}); err != nil {
		return err
	}
	log := k8szap.New()
	for _, item := range svcList.Items {
		if _, ok := systemNS[item.Namespace]; ok {
			continue
		}
		if err := k8sClient.Delete(ctx, &item); err != nil {
			if !apierrors.IsNotFound(err) {
				return err
			}
		}
		log.Info("Deleted service", "Namespace", item.Namespace, "Name", item.Name)
	}
	return nil
}
