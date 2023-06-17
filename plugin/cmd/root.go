package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"

	volumesnapshotv1 "github.com/kubernetes-csi/external-snapshotter/client/v6/apis/volumesnapshot/v1"
	crdv1beta1 "github.com/questdb/questdb-operator/api/v1beta1"
)

func init() {
	rootCmd.AddCommand(snapshotCmd)
	rootCmd.AddCommand(scheduleCmd)
	rootCmd.AddCommand(restoreCmd)
	rootCmd.AddCommand(questdbCommand)
}

var k8sClient client.Client

var rootCmd = &cobra.Command{
	Use:   "questdb",
	Short: "QuestDB Kubectl Plugin",
	Long:  "QuestDB Kubectl Plugin",
	PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
		// Initialize k8s client
		var (
			err error
			cfg *rest.Config
		)

		cfg, err = ctrl.GetConfig()
		if err != nil {
			return err
		}
		crdv1beta1.AddToScheme(scheme.Scheme)
		volumesnapshotv1.AddToScheme(scheme.Scheme)

		k8sClient, err = client.New(cfg, client.Options{
			Scheme: scheme.Scheme,
		})
		if err != nil {
			return err
		}

		return nil

	},
}

func Execute() error {
	return rootCmd.Execute()
}
