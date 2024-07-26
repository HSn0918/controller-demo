package controller

import (
	"context"
	"reflect"
	"slices"

	"appservice.com/utils"

	"k8s.io/apimachinery/pkg/util/sets"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/json"

	batchv1 "appservice.com/api/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

const (
	finalizerName = "appservice.batch.appservice.com"
	spec          = "spec"
)

// AppServiceReconciler reconciles a AppService object
type AppServiceReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=batch.appservice.com,resources=appservices,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch.appservice.com,resources=appservices/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch.appservice.com,resources=appservices/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the AppService object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.0/pkg/reconcile
func (r *AppServiceReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	instance := &batchv1.AppService{}
	if err := r.Client.Get(ctx, req.NamespacedName, instance); err != nil {
		if errors.IsNotFound(err) {
			return ctrl.Result{}, nil
		}
		return ctrl.Result{}, err
	}
	if !instance.DeletionTimestamp.IsZero() {
		if slices.Contains(instance.Finalizers, finalizerName) {
			if err := r.deleteAssociatedResources(ctx, instance); err != nil {
				return ctrl.Result{}, err
			}
			// 回收成功 删除finalize
			merge := client.MergeFrom(instance.DeepCopy())
			instance.Finalizers = sets.NewString(instance.Finalizers...).Delete(finalizerName).UnsortedList()
			if err := r.Client.Patch(ctx, instance, merge); err != nil {
				return ctrl.Result{}, err
			}
		}
		return ctrl.Result{}, nil
	}
	deployment := &appsv1.Deployment{}
	// 1. 不存在，则创建
	if err := r.Client.Get(ctx, req.NamespacedName, deployment); err != nil {
		// 如果不是不存在报错 返回
		if !errors.IsNotFound(err) {
			return ctrl.Result{}, err
		}
		// 1.不存在 则创建deployment
		deployment = NewDeployment(instance)
		if err := r.Client.Create(ctx, deployment); err != nil {
			return ctrl.Result{}, err
		}
		// 2.创建 Service
		svc := NewService(instance)
		if err := r.Client.Create(ctx, svc); err != nil {
			return ctrl.Result{}, err
		}
	} else {
		// 2. 存在，则对比spec
		utils.EnsureMapFieldsInitializedBFS(deployment)
		// 确保spec注释存在且为有效的JSON
		specAnnotation, exists := instance.Annotations[spec]
		if !exists || specAnnotation == "" {
			specAnnotation = "{}" // 设置为默认空的JSON对象
		}
		// 保留spec到oldSpec
		oldSpec := &batchv1.AppServiceSpec{}
		if err := json.Unmarshal([]byte(specAnnotation), oldSpec); err != nil {
			return ctrl.Result{}, err
		}
		if !reflect.DeepEqual(instance.Spec, *oldSpec) {
			newDeployment := NewDeployment(instance)
			currDeployment := &appsv1.Deployment{}
			if err := r.Client.Get(ctx, req.NamespacedName, currDeployment); err != nil {
				return ctrl.Result{}, err
			}
			currDeployment.Spec = newDeployment.Spec
			if err := r.Client.Update(ctx, currDeployment); err != nil {
				return ctrl.Result{}, err
			}

			newService := NewService(instance)
			currService := &corev1.Service{}
			if err := r.Client.Get(ctx, req.NamespacedName, currService); err != nil {
				return ctrl.Result{}, err
			}

			currIP := currService.Spec.ClusterIP
			currService.Spec = newService.Spec
			currService.Spec.ClusterIP = currIP
			if err := r.Client.Update(ctx, currService); err != nil {
				return ctrl.Result{}, err
			}
		}
	}
	if !slices.Contains(instance.Finalizers, finalizerName) {
		merge := client.MergeFrom(instance.DeepCopy())
		instance.Finalizers = append(instance.Finalizers, finalizerName)
		if err := r.Client.Patch(ctx, instance, merge); err != nil {
			return ctrl.Result{}, err
		}
	}
	// 3. 关联 Annotations
	data, _ := json.Marshal(instance.Spec)
	if instance.Annotations != nil {
		instance.Annotations[spec] = string(data)
	} else {
		instance.Annotations = map[string]string{spec: string(data)}
	}
	if err := r.Client.Update(ctx, instance); err != nil {
		return ctrl.Result{}, err
	}
	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *AppServiceReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&batchv1.AppService{}).
		Complete(r)
}

// controllers/appservice_controller.go
func NewDeployment(app *batchv1.AppService) *appsv1.Deployment {
	labels := map[string]string{"app": app.Name}
	selector := &metav1.LabelSelector{MatchLabels: labels}
	return &appsv1.Deployment{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "apps/v1",
			Kind:       "Deployment",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, schema.GroupVersionKind{
					Group:   batchv1.GroupVersion.Group,
					Version: batchv1.GroupVersion.Version,
					Kind:    app.Kind,
				}),
			},
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: app.Spec.Replicas,
			Selector: selector,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{Containers: newContainer(app)},
			},
		},
	}
}

// controllers/appservice_controller.go
func newContainer(app *batchv1.AppService) []corev1.Container {
	containerPorts := []corev1.ContainerPort{}
	for _, svcPort := range app.Spec.Ports {
		cport := corev1.ContainerPort{}
		cport.ContainerPort = svcPort.TargetPort.IntVal
		containerPorts = append(containerPorts, cport)
	}
	return []corev1.Container{
		{
			Name:            app.Name,
			Image:           app.Spec.Image,
			Resources:       app.Spec.Resources,
			Ports:           containerPorts,
			ImagePullPolicy: corev1.PullIfNotPresent,
			Env:             app.Spec.Envs,
		},
	}
}

// controllers/appservice_controller.go
func NewService(app *batchv1.AppService) *corev1.Service {
	return &corev1.Service{
		TypeMeta: metav1.TypeMeta{
			Kind:       "Service",
			APIVersion: "v1",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      app.Name,
			Namespace: app.Namespace,
			OwnerReferences: []metav1.OwnerReference{
				*metav1.NewControllerRef(app, schema.GroupVersionKind{
					Group:   batchv1.GroupVersion.Group,
					Version: batchv1.GroupVersion.Version,
					Kind:    app.Kind,
				}),
			},
		},
		Spec: corev1.ServiceSpec{
			Type:  corev1.ServiceTypeNodePort,
			Ports: app.Spec.Ports,
			Selector: map[string]string{
				"app": app.Name,
			},
		},
	}
}

func (r *AppServiceReconciler) deleteAssociatedResources(ctx context.Context, app *batchv1.AppService) error {
	deployment := &appsv1.Deployment{}
	// 没有err，说明找到了deployment，需要删除
	if err := r.Client.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, deployment); err == nil {
		if err := r.Client.Delete(ctx, deployment); err != nil {
			return err
		}
	}

	svc := &corev1.Service{}
	// 没有err，说明找到了service，需要删除
	if err := r.Client.Get(ctx, client.ObjectKey{Name: app.Name, Namespace: app.Namespace}, svc); err == nil {
		if err := r.Client.Delete(ctx, svc); err != nil {
			return err
		}
	}
	return nil
}
func ensureInitialized(currDeployment *appsv1.Deployment) {
	if currDeployment.Spec.Template.Labels == nil {
		currDeployment.Spec.Template.Labels = map[string]string{}
	}
	if currDeployment.Spec.Template.Annotations == nil {
		currDeployment.Spec.Template.Annotations = map[string]string{}
	}
	if currDeployment.Spec.Selector == nil {
		currDeployment.Spec.Selector = &metav1.LabelSelector{}
	}
	if currDeployment.Spec.Template.Spec.Containers == nil {
		currDeployment.Spec.Template.Spec.Containers = []corev1.Container{}
	}
	if currDeployment.Spec.Template.Spec.InitContainers == nil {
		currDeployment.Spec.Template.Spec.InitContainers = []corev1.Container{}
	}
	if currDeployment.Spec.Template.Spec.Volumes == nil {
		currDeployment.Spec.Template.Spec.Volumes = []corev1.Volume{}
	}
	if currDeployment.ObjectMeta.OwnerReferences == nil {
		currDeployment.ObjectMeta.OwnerReferences = []metav1.OwnerReference{}
	}
}
