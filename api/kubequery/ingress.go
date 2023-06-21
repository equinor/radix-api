package kubequery

import (
	"context"

	operatorUtils "github.com/equinor/radix-operator/pkg/apis/utils"
	"github.com/equinor/radix-operator/pkg/apis/utils/labels"
	"golang.org/x/sync/errgroup"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// GetIngressesForEnvironments returns all HorizontalPodAutoscalers for the specified application and environments.
// Go routines are used to query HPAs per environment, and number of concurrenct Go routines is controlled with the parallelism parameter.
func GetIngressesForEnvironments(ctx context.Context, client kubernetes.Interface, appName string, envNames []string, parallelism int) ([]networkingv1.Ingress, error) {
	if len(envNames) == 0 {
		return nil, nil
	}
	ch := make(chan []networkingv1.Ingress, len(envNames))
	var g errgroup.Group
	g.SetLimit(parallelism)

	for _, envName := range envNames {
		g.Go(func(envName string) func() error {
			return func() error {
				ingresses, err := getIngressesForEnvironment(ctx, client, appName, envName)
				if err != nil {
					return err
				}
				ch <- ingresses
				return nil
			}
		}(envName))
	}

	err := g.Wait()
	close(ch)
	if err != nil {
		return nil, err
	}
	var ingressList []networkingv1.Ingress
	for ingresses := range ch {
		ingressList = append(ingressList, ingresses...)
	}
	return ingressList, nil
}

func getIngressesForEnvironment(ctx context.Context, client kubernetes.Interface, appName, envName string) ([]networkingv1.Ingress, error) {
	ns := operatorUtils.GetEnvironmentNamespace(appName, envName)
	ingresses, err := client.NetworkingV1().Ingresses(ns).List(ctx, metav1.ListOptions{LabelSelector: labels.ForApplicationName(appName).String()})
	if err != nil {
		return nil, err
	}
	return ingresses.Items, nil
}
