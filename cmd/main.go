package main

import (
	"os"
	"time"

	testresourceclientset "github.com/qibobo/test-controller/lib/testresource/generated/clientset/versioned"
	testresourecescheme "github.com/qibobo/test-controller/lib/testresource/generated/clientset/versioned/scheme"
	testresourceinformers "github.com/qibobo/test-controller/lib/testresource/generated/informers/externalversions"
	testresourcelisters "github.com/qibobo/test-controller/lib/testresource/generated/listers/testresource/v1beta1"
	testresourcev1beta1 "github.com/qibobo/test-controller/lib/testresource/v1beta1"
	corev1 "k8s.io/api/core/v1"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"
)

type Controller struct {
	kubeclientset          kubernetes.Interface
	apiextensionsclientset apiextensionsclientset.Interface
	testresourceclientset  testresourceclientset.Interface
	informer               cache.SharedIndexInformer
	lister                 testresourcelisters.TestResourceLister
	recorder               record.EventRecorder
	workqueue              workqueue.RateLimitingInterface
}

func NewController() *Controller {
	kubeconfig := os.Getenv("KUBECONFIG")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf(err.Error())
	}
	kubeClient := kubernetes.NewForConfigOrDie(config)
	apiextensionsClient := apiextensionsclientset.NewForConfigOrDie(config)
	testClient := testresourceclientset.NewForConfigOrDie(config)

	informerFactory := testresourceinformers.NewSharedInformerFactory(testClient,
		time.Hour)
	informer := informerFactory.Qibobo().V1beta1().TestResources()
	informer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: func(obj interface{}) {
			klog.Infof("Added %v", obj)
		},
		UpdateFunc: func(oldObj, newObj interface{}) {
			klog.Infof("Updated %v", newObj)
		},
		DeleteFunc: func(obj interface{}) {
			klog.Infof("Deleted %v", obj)
		},
	})
	informerFactory.Start(wait.NeverStop)

	utilruntime.Must(testresourcev1beta1.AddToScheme(testresourecescheme.Scheme))
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(klog.Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClient.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(testresourecescheme.Scheme, corev1.EventSource{Component: "testresource-controller"})
	workqueue := workqueue.NewRateLimitingQueue(workqueue.DefaultControllerRateLimiter())

	return &Controller{
		kubeclientset:          kubeClient,
		apiextensionsclientset: apiextensionsClient,
		testresourceclientset:  testClient,
		informer:               informer.Informer(),
		lister:                 informer.Lister(),
		recorder:               recorder,
		workqueue:              workqueue,
	}
}

func (c *Controller) Run() {
	defer utilruntime.HandleCrash()
	defer c.workqueue.ShutDown()
	klog.Infoln("Waiting cache to be synced.")

	timeout := time.NewTimer(time.Second * 30)
	timeoutCh := make(chan struct{})
	go func() {
		<-timeout.C
		timeoutCh <- struct{}{}
	}()
	if ok := cache.WaitForCacheSync(timeoutCh, c.informer.HasSynced); !ok {
		klog.Fatalln("Timeout expired during wait for caches to sync.")
	}
	klog.Infoln("Starting custom controller")
}

func main() {
	controller := NewController()
	controller.Run()
}
