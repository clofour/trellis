package main

import (
	"fmt"

	"github.com/clofour/trellis/internal/client"
	"github.com/spf13/cobra"
)

func NewServicesCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "services",
		Short: "List healthy services in the cluster",
		RunE: func(cmd *cobra.Command, args []string) error {
			services, err := client.NewNamespaceServerClient(config.ClusterToken, config.ServerAddr, config.Namespace).ListServices(cmd.Context())
			if err != nil {
				return err
			}
			for _, s := range *services {
				labels := ""
				for k, v := range s.Labels {
					if labels != "" {
						labels += ","
					}
					labels += k + "=" + v
				}
				ports := ""
				for _, p := range s.Ports {
					if ports != "" {
						ports += ","
					}
					ports += fmt.Sprintf("%d->%d", p.HostPort, p.ContainerPort)
				}
				fmt.Fprintf(cmd.OutOrStdout(), "%s\t%s/%s\t%s\t%s\t%s\n", s.ID, s.Job, s.Group, s.Address, ports, labels)
			}
			return nil
		},
	}
}
