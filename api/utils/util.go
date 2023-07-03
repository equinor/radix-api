package utils

import (
	"context"
	"github.com/equinor/radix-api/models"
	corev1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func UserIsAdmin(ctx context.Context, user *models.Account, appName string) (bool, error) {
	review, err := user.Client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &corev1.SelfSubjectAccessReview{
		Spec: corev1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &corev1.ResourceAttributes{
				Verb:     "patch",
				Group:    "radix.equinor.com",
				Resource: "radixregistrations",
				Name:     appName,
			},
		},
	}, metav1.CreateOptions{})
	return review.Status.Allowed, err
}
