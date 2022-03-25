/*
Copyright 2021 Red Hat

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package common

import (
	"context"

	batchv1 "k8s.io/api/batch/v1"
	k8s_errors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"errors"
)

// DeleteJob func
// kclient required to properly cleanup the job depending pods with DeleteOptions
func DeleteJob(
	ctx context.Context,
	job *batchv1.Job,
	kclient kubernetes.Interface,
	log logr.Logger,
) (bool, error) {

	// Check if this Job already exists
	foundJob, err := kclient.BatchV1().Jobs(job.Namespace).Get(ctx, job.Name, metav1.GetOptions{})
	if err == nil {
		log.Info("Deleting Job", "Job.Namespace", job.Namespace, "Job.Name", job.Name)
		background := metav1.DeletePropagationBackground
		err = kclient.BatchV1().Jobs(foundJob.Namespace).Delete(ctx, foundJob.Name, metav1.DeleteOptions{PropagationPolicy: &background})
		if err != nil {
			return false, err
		}
		return true, err
	}
	return false, nil
}

// WaitOnJob func
func WaitOnJob(
	ctx context.Context,
	job *batchv1.Job,
	client client.Client,
	log logr.Logger,
) (bool, error) {
	// Check if this Job already exists
	foundJob := &batchv1.Job{}
	err := client.Get(ctx, types.NamespacedName{Name: job.Name, Namespace: job.Namespace}, foundJob)
	if err != nil {
		log.Info("WaitOnJob err")
		return true, err
	} else if foundJob != nil {
		if foundJob.Status.Active > 0 {
			log.Info("Job Status Active... requeuing")
			return true, err
		} else if foundJob.Status.Failed > 0 {
			log.Info("Job Status Failed")
			return true, k8s_errors.NewInternalError(errors.New("Job Failed. Check job logs"))
		} else if foundJob.Status.Succeeded > 0 {
			log.Info("Job Status Successful")
		} else {
			log.Info("Job Status incomplete... requeuing")
			return true, err
		}
	}
	return false, nil

}
