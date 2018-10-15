package platform

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"github.com/statoil/radix-operator/pkg/client/clientset/versioned/fake"
	batchv1 "k8s.io/api/batch/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kubereal "k8s.io/client-go/kubernetes"
	kubernetes "k8s.io/client-go/kubernetes/fake"
)

func TestIsDeployKeyValid(t *testing.T) {
	radixclient := fake.NewSimpleClientset()
	kubeclient := kubernetes.NewSimpleClientset()

	t.Run("job succeeded", func(t *testing.T) {
		isValid, err := runIsDeployKeyValid(kubeclient, radixclient,
			func(job batchv1.Job) batchv1.Job {
				job.Status.Succeeded =
					int32(1)
				return job
			})

		assert.True(t, isValid)
		assert.Nil(t, err)
	})

	t.Run("missing rr", func(t *testing.T) {
		isValid, err := IsDeployKeyValid(kubeclient, radixclient, "some-app")
		assert.False(t, isValid)
		assert.NotNil(t, err)
	})

	t.Run("job failed", func(t *testing.T) {
		isValid, err := runIsDeployKeyValid(kubeclient, radixclient,
			func(job batchv1.Job) batchv1.Job {
				job.Status.Failed =
					int32(1)
				return job
			})

		assert.False(t, isValid)
		assert.NotNil(t, err)
	})
}

func runIsDeployKeyValid(kubeclient kubereal.Interface, radixclient radixclient.Interface, setJobStatus func(batchv1.Job) batchv1.Job) (bool, error) {
	anyApp := NewBuilder().withName("some-app").withRepository("https://github.com/Equinor/some-app").BuildRegistration()
	HandleCreateRegistation(radixclient, *anyApp)

	finish := make(chan bool)
	go func() {
		time.Sleep(200 * time.Millisecond)
		jobs, _ := kubeclient.BatchV1().Jobs("some-app-app").List(metav1.ListOptions{})
		job := jobs.Items[0]
		job = setJobStatus(job)
		kubeclient.BatchV1().Jobs("some-app-app").Update(&job)
		finish <- true
	}()

	isValid, err := IsDeployKeyValid(kubeclient, radixclient, "some-app")

	<-finish
	return isValid, err
}
