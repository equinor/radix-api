package kubequery

import (
	"context"

	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	radixclient "github.com/equinor/radix-operator/pkg/client/clientset/versioned"
	"golang.org/x/sync/errgroup"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetRadixDeploymentsForEnvironments(ctx context.Context, client radixclient.Interface, appName string, envNames []string, parallelism int) ([]radixv1.RadixDeployment, error) {
	if len(envNames) == 0 {
		return nil, nil
	}
	ch := make(chan []radixv1.RadixDeployment, len(envNames))
	var g errgroup.Group
	g.SetLimit(parallelism)

	for _, envName := range envNames {
		g.Go(func(envName string) func() error {
			return func() error {
				rds, err := getRadixDeploymentsForEnvironment(ctx, client, appName, envName)
				if err != nil {
					return err
				}
				ch <- rds
				return nil
			}
		}(envName))
	}

	err := g.Wait()
	close(ch)
	if err != nil {
		return nil, err
	}
	var rdList []radixv1.RadixDeployment
	for rd := range ch {
		rdList = append(rdList, rd...)
	}
	return rdList, nil
}

func getRadixDeploymentsForEnvironment(ctx context.Context, client radixclient.Interface, appName, envName string) ([]radixv1.RadixDeployment, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	rds, err := client.RadixV1().RadixDeployments(ns).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	return rds.Items, nil
}
