package authorizationvalidator

import (
	"context"
	"github.com/equinor/radix-api/models"
	"github.com/google/uuid"
	corev1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var defaultValidator = validator{}

func DefaultValidator() Interface {
	return &defaultValidator
}

type validator struct{}

func (v *validator) UserIsAdmin(ctx context.Context, user *models.Account, appName string) (bool, error) {
	review, err := user.Client.AuthorizationV1().SelfSubjectAccessReviews().Create(ctx, &corev1.SelfSubjectAccessReview{
		ObjectMeta: metav1.ObjectMeta{
			Name: uuid.New().String(),
		},
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
