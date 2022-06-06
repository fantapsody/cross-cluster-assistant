package service_syncer

import (
	"context"
	"fmt"
	"github.com/go-logr/logr"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"reflect"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
)

type ServiceSyncerReconciler struct {
	client.Client
	DstClient client.Client
	Log       logr.Logger
}

var _ reconcile.Reconciler = &ServiceSyncerReconciler{}

var systemNS = map[string]string{
	"kube-system": "",
	"kube-public": "",
	"sn-system":   "",
}

const (
	LabelManaged               = "k8s.sn.io/managed-by"
	LabelCrossClusterAssistant = "cross-cluster-assistant"
)

func (s *ServiceSyncerReconciler) Reconcile(ctx context.Context, request reconcile.Request) (reconcile.Result, error) {
	// do not copy services in system namespaces
	if _, ok := systemNS[request.Namespace]; ok {
		return reconcile.Result{}, nil
	}

	log := s.Log.WithValues("Namespace", request.Namespace, "Name", request.Name)

	svc := &corev1.Service{}
	err := s.Get(ctx, types.NamespacedName{
		Namespace: request.Namespace,
		Name:      request.Name,
	}, svc)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return reconcile.Result{}, nil
		}
		return reconcile.Result{}, err
	}

	// headless service with selectors only
	if svc.Spec.ClusterIP != corev1.ClusterIPNone || len(svc.Spec.Selector) == 0 {
		return reconcile.Result{}, nil
	}

	if err = s.reconcileService(ctx, log, svc); err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}

func (s *ServiceSyncerReconciler) reconcileService(ctx context.Context,
	log logr.Logger, svc *corev1.Service) error {
	dstSvc := &corev1.Service{}
	err := s.DstClient.Get(ctx, types.NamespacedName{
		Namespace: svc.Namespace,
		Name:      svc.Name,
	}, dstSvc)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		dstSvc = makeDstService(svc)
		if err = s.DstClient.Create(ctx, dstSvc); err != nil {
			return nil
		}
		log.Info("Created headless service in the destination cluster")
	} else {
		if v, ok := dstSvc.Labels[LabelManaged]; !ok || v != LabelCrossClusterAssistant {
			return fmt.Errorf("service not managed by CCA")
		}
		desiredSvc := makeDstService(svc)
		if !reflect.DeepEqual(dstSvc.Spec, desiredSvc.Spec) {
			dstSvc.Spec = desiredSvc.Spec
			if err = s.DstClient.Update(ctx, dstSvc); err != nil {
				return nil
			}
			log.Info("Updated headless service in the destination cluster")
		}
	}

	if err = s.reconcileEndpoint(ctx, log, svc); err != nil {
		return err
	}
	log.Info("Reconciled")
	return nil
}

func (s *ServiceSyncerReconciler) reconcileEndpoint(ctx context.Context,
	log logr.Logger, svc *corev1.Service) error {
	name := types.NamespacedName{
		Namespace: svc.Namespace,
		Name:      svc.Name,
	}
	srcEp := &corev1.Endpoints{}
	err := s.Get(ctx, name, srcEp)
	if err != nil {
		return err
	}

	dstEp := &corev1.Endpoints{}
	err = s.DstClient.Get(ctx, name, dstEp)
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}
		dstEp = makeDstEndpoint(srcEp)
		if err = s.DstClient.Create(ctx, dstEp); err != nil {
			return err
		}
		log.Info("Created endpoints in the destination cluster")
	} else {
		if v, ok := dstEp.Labels[LabelManaged]; !ok || v != LabelCrossClusterAssistant {
			return fmt.Errorf("endpoints not managed by CCA")
		}
		if !reflect.DeepEqual(srcEp.Subsets, dstEp.Subsets) {
			desiredEp := makeDstEndpoint(srcEp)
			dstEp.Subsets = desiredEp.Subsets
			if err = s.DstClient.Update(ctx, dstEp); err != nil {
				return err
			}
			log.Info("Updated endpoints in the destination cluster")
		}
	}

	return nil
}

func makeDstService(svc *corev1.Service) *corev1.Service {
	spec := svc.Spec.DeepCopy()
	spec.Selector = nil
	return &corev1.Service{
		ObjectMeta: v1.ObjectMeta{
			Namespace: svc.Namespace,
			Name:      svc.Name,
			Labels:    makeLabels(),
		},
		Spec: *spec,
	}
}

func makeDstEndpoint(ep *corev1.Endpoints) *corev1.Endpoints {
	return &corev1.Endpoints{
		ObjectMeta: v1.ObjectMeta{
			Namespace: ep.Namespace,
			Name:      ep.Name,
			Labels:    makeLabels(),
		},
		Subsets: ep.DeepCopy().Subsets,
	}
}

func makeLabels() map[string]string {
	return map[string]string{
		LabelManaged: LabelCrossClusterAssistant,
	}
}
