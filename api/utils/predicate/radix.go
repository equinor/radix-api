package predicate

import radixv1 "github.com/equinor/radix-operator/pkg/apis/radix/v1"

func IsActiveRadixDeployment(rd radixv1.RadixDeployment) bool {
	return rd.Status.Condition == radixv1.DeploymentActive
}

func IsRadixDeploymentForAppAndEnv(appName, envName string) func(rd radixv1.RadixDeployment) bool {
	return func(rd radixv1.RadixDeployment) bool {
		return rd.Spec.AppName == appName && rd.Spec.Environment == envName
	}
}

func IsRadixDeploymentForRadixBatch(batch *radixv1.RadixBatch) func(rd radixv1.RadixDeployment) bool {
	return func(rd radixv1.RadixDeployment) bool {
		if batch == nil {
			return false
		}
		return batch.Spec.RadixDeploymentJobRef.Name == rd.Name && batch.Namespace == rd.Namespace
	}
}

func IsRadixDeployJobComponentWithName(name string) func(deployJob radixv1.RadixDeployJobComponent) bool {
	return func(deployJob radixv1.RadixDeployJobComponent) bool {
		return deployJob.Name == name
	}
}

func IsNotOrphanEnvironment(re radixv1.RadixEnvironment) bool {
	return !IsOrphanEnvironment(re)
}

func IsOrphanEnvironment(re radixv1.RadixEnvironment) bool {
	return re.Status.Orphaned || re.Status.OrphanedTimestamp != nil
}

func IsBatchJobStatusForJobName(name string) func(jobStatus radixv1.RadixBatchJobStatus) bool {
	return func(jobStatus radixv1.RadixBatchJobStatus) bool {
		return jobStatus.Name == name
	}
}

func IsBatchJobWithName(name string) func(job radixv1.RadixBatchJob) bool {
	return func(job radixv1.RadixBatchJob) bool {
		return job.Name == name
	}
}
