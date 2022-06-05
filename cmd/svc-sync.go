package cmd

import (
	"context"
	service_syncer "cross-cluster-assistant/pkg/service-syncer"
	"fmt"
	"github.com/spf13/cobra"
	"go.uber.org/zap/zapcore"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	k8szap "sigs.k8s.io/controller-runtime/pkg/log/zap"
	"sigs.k8s.io/controller-runtime/pkg/manager/signals"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

type CmdSyncSvc struct {
	contextA string
	contextB string
}

func (c *CmdSyncSvc) RunE(cmd *cobra.Command, args []string) error {
	var encoder zapcore.Encoder
	controllerruntime.SetLogger(k8szap.New(k8szap.Encoder(encoder)))
	ch := make(chan error)
	ctx := signals.SetupSignalHandler()
	go func() {
		err := syncServices(ctx, c.contextA, c.contextB)
		ch <- err
	}()
	go func() {
		err := syncServices(ctx, c.contextB, c.contextA)
		ch <- err
	}()
	err := <- ch
	if err != nil {
		return err
	}
	return nil
}

func AddSyncSvcCmd(root *cobra.Command) {
	css := &CmdSyncSvc{
	}
	cmd := &cobra.Command{
		Use: "sync-svc",
		RunE: css.RunE,
	}
	cmd.Flags().StringVar(&css.contextA, "a", "", "")
	cmd.Flags().StringVar(&css.contextB, "b", "", "")

	root.AddCommand(cmd)
}

func syncServices(ctx context.Context, ctxSrc, ctxDst string) error {
	if ctxSrc == "" {
		return fmt.Errorf("the source context is empty")
	}
	if ctxDst == "" {
		return fmt.Errorf("the destination context is empty")
	}
	configSrc, err := config.GetConfigWithContext(ctxSrc)
	if err != nil {
		return err
	}
	configDst, err := config.GetConfigWithContext(ctxDst)
	if err != nil {
		return err
	}
	mgr, err := controllerruntime.NewManager(configSrc, controllerruntime.Options{
		MetricsBindAddress: "0",
	})
	if err != nil {
		return err
	}
	dstClient, err := client.New(configDst, client.Options{
	})
	if err != nil {
		return err
	}
	reconciler := &service_syncer.ServiceSyncerReconciler{
		Client: mgr.GetClient(),
		DstClient: dstClient,
		Log: controllerruntime.Log.WithName("ServiceSyncerController").
			WithValues("From", ctxSrc, "To", ctxDst),
	}
	err = controllerruntime.NewControllerManagedBy(mgr).
		For(&corev1.Service{}).
		Watches(&source.Kind{Type: &corev1.Endpoints{}}, handler.EnqueueRequestsFromMapFunc(func(object client.Object) []reconcile.Request {
			return []reconcile.Request{
				{
					NamespacedName: types.NamespacedName{
						Namespace: object.GetNamespace(),
						Name: object.GetName(),
					},
				},
			}
	})).
		Complete(reconciler)
	if err != nil {
		return err
	}
	if err = mgr.Start(ctx); err != nil {
		return err
	}
	return nil
}
