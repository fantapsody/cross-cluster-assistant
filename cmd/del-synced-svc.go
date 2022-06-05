package cmd

import (
	"context"
	service_syncer "cross-cluster-assistant/pkg/service-syncer"
	"fmt"
	"github.com/spf13/cobra"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/config"
)

type CmdDelSyncedSvc struct {
	context string
}

func (c *CmdDelSyncedSvc) RunE(cmd *cobra.Command, args []string) error {
	return delSyncedServices(context.Background(), c.context)
}

func AddDelSyncedSvcCmd(root *cobra.Command) {
	css := &CmdDelSyncedSvc{
	}
	cmd := &cobra.Command{
		Use: "del-synced-svc",
		RunE: css.RunE,
	}
	cmd.Flags().StringVar(&css.context, "ctx", "", "")

	root.AddCommand(cmd)
}

func delSyncedServices(ctx context.Context, k8sCtx string) error {
	if k8sCtx == "" {
		return fmt.Errorf("the k8s context is empty")
	}
	k8sConfig, err := config.GetConfigWithContext(k8sCtx)
	if err != nil {
		return err
	}
	k8sClient, err := client.New(k8sConfig, client.Options{
	})
	if err != nil {
		return err
	}
	return service_syncer.DelSyncedServices(ctx, k8sClient)
}
