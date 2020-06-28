package proxyvalidatingtype

import (
	"github.com/redislabs/gesher/pkg/common"
	"io/ioutil"
	"k8s.io/api/admissionregistration/v1beta1"
	"path/filepath"

	appv1alpha1 "github.com/redislabs/gesher/pkg/apis/app/v1alpha1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_proxyvalidatingtype")

// Add creates a new ProxyValidatingType Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager) error {
	var err error

	// set caBundle should be already created in main()
	caBundle, err = ioutil.ReadFile(filepath.Join(common.CertDir, common.CertPem))
	if err != nil {
		return err
	}

	return add(mgr, newReconciler(mgr))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager) reconcile.Reconciler {
	return &ReconcileProxyValidatingType{client: mgr.GetClient(), scheme: mgr.GetScheme()}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("proxyvalidatingtype-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ProxyValidatingType
	err = c.Watch(&source.Kind{Type: &appv1alpha1.ProxyValidatingType{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// Watch for changes to secondary resource ValidatingWebhookConfiguration and requeue the owner ProxyValidatingType
	// TODO: Need to figure out how to "own" the type we make when its not really tied to an individual resource
	err = c.Watch(&source.Kind{Type: &v1beta1.ValidatingWebhookConfiguration{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &appv1alpha1.ProxyValidatingType{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileProxyValidatingType implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileProxyValidatingType{}

// ReconcileProxyValidatingType reconciles a ProxyValidatingType object
type ReconcileProxyValidatingType struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
}

// Reconcile reads that state of the cluster for a ProxyValidatingType object and makes changes based on the state read
// and what is in the ProxyValidatingType.Spec

// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileProxyValidatingType) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling ProxyValidatingType")

	observedState, err := observe(r.client, request, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	if observedState == nil {
		return reconcile.Result{}, nil
	}

	analyzedState, err := analyze(observedState, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	err = act(r.client, analyzedState, reqLogger)
	if err != nil {
		return reconcile.Result{}, err
	}

	return reconcile.Result{}, nil
}