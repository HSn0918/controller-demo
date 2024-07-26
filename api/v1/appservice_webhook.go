/*
Copyright 2024 appservice.

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

package v1

import (
	"regexp"
	"strings"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

const (
	colon        = ":"
	leastVersion = ":least"
	versionReg   = `^\d+\.\d+\.\d+$`
)

var (
	defaultReplicas = int32(1)
)

// log is for logging in this package.
var appservicelog = logf.Log.WithName("appservice-resource")

// SetupWebhookWithManager will setup the manager to manage the webhooks
func (r *AppService) SetupWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).
		For(r).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-batch-appservice-com-v1-appservice,mutating=true,failurePolicy=fail,sideEffects=None,groups=batch.appservice.com,resources=appservices,verbs=create;update,versions=v1,name=mappservice.kb.io,admissionReviewVersions=v1

var _ webhook.Defaulter = &AppService{}

// Default implements webhook.Defaulter so a webhook will be registered for the type
func (r *AppService) Default() {
	appservicelog.Info("default", "name", r.Name)
	// 如果是要删除，就快速return
	if r.DeletionTimestamp.IsZero() {
		return
	}
	// 检查 Image 字段的最后一部分是否包含冒号（版本号）
	if lastColon := strings.LastIndex(r.Spec.Image, colon); lastColon == -1 {
		// 如果没有冒号，说明没有版本号，添加 ":latest"
		r.Spec.Image += leastVersion
	} else {
		// 从最后一个冒号位置开始检查是否有合法的版本号
		versionPart := r.Spec.Image[lastColon+1:]
		if !isValidVersion(versionPart) {
			r.Spec.Image += leastVersion
		}
	}
}
func isValidVersion(version string) bool {
	// 检查是否符合语义版本格式，例如 "1.0.0"
	matched, err := regexp.MatchString(versionReg, version)
	if err != nil {
		return false
	}
	return matched
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-batch-appservice-com-v1-appservice,mutating=false,failurePolicy=fail,sideEffects=None,groups=batch.appservice.com,resources=appservices,verbs=create;update,versions=v1,name=vappservice.kb.io,admissionReviewVersions=v1

var _ webhook.Validator = &AppService{}

// ValidateCreate implements webhook.Validator so a webhook will be registered for the type
func (r *AppService) ValidateCreate() (admission.Warnings, error) {
	appservicelog.Info("validate create", "name", r.Name)
	if !checkSpecReplicas(r.Spec.Replicas) {
		r.Spec.Replicas = &defaultReplicas
	}
	return nil, nil
}

// ValidateUpdate implements webhook.Validator so a webhook will be registered for the type
func (r *AppService) ValidateUpdate(old runtime.Object) (admission.Warnings, error) {
	appservicelog.Info("validate update", "name", r.Name)
	if !checkSpecReplicas(r.Spec.Replicas) {
		r.Spec.Replicas = &defaultReplicas
	}
	return nil, nil
}

// ValidateDelete implements webhook.Validator so a webhook will be registered for the type
func (r *AppService) ValidateDelete() (admission.Warnings, error) {
	appservicelog.Info("validate delete", "name", r.Name)

	// TODO(user): fill in your validation logic upon object deletion.
	return nil, nil
}
func checkSpecReplicas(i *int32) bool {
	if i == nil {
		return false // 添加了对 nil 指针的检查
	}
	if *i < 0 {
		return false
	}
	return true
}
