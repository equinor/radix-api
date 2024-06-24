package kubequery_test

import (
	"context"
	"fmt"
	"testing"

	"github.com/equinor/radix-api/api/kubequery"
	"github.com/equinor/radix-operator/pkg/apis/kube"
	radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"
	radixlabels "github.com/equinor/radix-operator/pkg/apis/utils/labels"
	radixfake "github.com/equinor/radix-operator/pkg/client/clientset/versioned/fake"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func Test_GetRadixBatchesForJobComponent(t *testing.T) {
	app, env, comp := "app1", "env1", "c1"

	ns := func(app, env string) string { return fmt.Sprintf("%s-%s", app, env) }
	matchjob1 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "matchjob1",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	matchjob2 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "matchjob2",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	matchbatch1 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "matchbatch1",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeBatch)),
	}}
	unmatched1 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched1",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName("othercomp"), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched2 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched2",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched3 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched3",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched4 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched4",
		Namespace: ns(app, "otherenv"),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp), radixlabels.ForBatchType(kube.RadixBatchTypeJob)),
	}}
	unmatched5 := radixv1.RadixBatch{ObjectMeta: metav1.ObjectMeta{
		Name:      "unmatched5",
		Namespace: ns(app, env),
		Labels:    radixlabels.Merge(radixlabels.ForApplicationName(app), radixlabels.ForComponentName(comp)),
	}}

	client := radixfake.NewSimpleClientset()
	client.RadixV1().RadixBatches(matchjob1.Namespace).Create(context.Background(), &matchjob1, metav1.CreateOptions{})
	client.RadixV1().RadixBatches(matchjob2.Namespace).Create(context.Background(), &matchjob2, metav1.CreateOptions{})
	client.RadixV1().RadixBatches(matchbatch1.Namespace).Create(context.Background(), &matchbatch1, metav1.CreateOptions{})
	client.RadixV1().RadixBatches(unmatched1.Namespace).Create(context.Background(), &unmatched1, metav1.CreateOptions{})
	client.RadixV1().RadixBatches(unmatched2.Namespace).Create(context.Background(), &unmatched2, metav1.CreateOptions{})
	client.RadixV1().RadixBatches(unmatched3.Namespace).Create(context.Background(), &unmatched3, metav1.CreateOptions{})
	client.RadixV1().RadixBatches(unmatched4.Namespace).Create(context.Background(), &unmatched4, metav1.CreateOptions{})
	client.RadixV1().RadixBatches(unmatched5.Namespace).Create(context.Background(), &unmatched5, metav1.CreateOptions{})

	// Get batches of type job
	actual, err := kubequery.GetRadixBatchesForJobComponent(context.Background(), client, app, env, comp, kube.RadixBatchTypeJob)
	require.NoError(t, err)
	expected := []radixv1.RadixBatch{matchjob1, matchjob2}
	assert.ElementsMatch(t, expected, actual)

	// Get batches of type batch
	actual, err = kubequery.GetRadixBatchesForJobComponent(context.Background(), client, app, env, comp, kube.RadixBatchTypeBatch)
	require.NoError(t, err)
	expected = []radixv1.RadixBatch{matchbatch1}
	assert.ElementsMatch(t, expected, actual)
}
