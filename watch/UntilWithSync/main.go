package main

import (
	"context"
	"flag"
	"log"
	"path/filepath"
	"time"

	"github.com/ray31245/tryk8sclient/util"
	batchv1 "k8s.io/api/batch/v1"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	watchtools "k8s.io/client-go/tools/watch"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var namespace *string
	var name *string
	var number *int
	var kubeconfig *string
	var isJob *bool

	namespace = flag.String("namespace", "default", "choose namespace to select")
	name = flag.String("name", "", "select pod or job by name")
	number = flag.Int("number", 0, "numbers of repeat listen")
	kubeconfig = flag.String("kubeconfig", filepath.Join(homedir.HomeDir(), ".kube", "config"), "<HomeDir>/kube/config")
	isJob = flag.Bool("isJob", false, "if set true watch job, otherwise watch pob")
	flag.Parse()

	clientSet, err := util.GetClient(*kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var obj runtime.Object
	if *isJob {
		obj, err = clientSet.BatchV1().Jobs(*namespace).Get(ctx, *name, metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}
	} else {
		obj, err = clientSet.CoreV1().Pods(*namespace).Get(ctx, *name, metav1.GetOptions{})
		if err != nil {
			panic(err.Error())
		}
	}

	for i := 1; i <= *number; i++ {
		go watchEvent(ctx, clientSet, obj, i)
	}
	for {
		time.Sleep(time.Second * 10)
	}
}

func watchEvent(ctx context.Context, clientSet kubernetes.Interface, objType runtime.Object, number int) {
	var name string
	var listF func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error)
	var watchF func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error)

	pod, ok := objType.(*v1.Pod)
	if ok {
		name = pod.Name
		listF = func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
			return clientSet.CoreV1().Pods(pod.Namespace).List(ctx, opts)
		}
		watchF = func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			return clientSet.CoreV1().Pods(pod.Namespace).Watch(ctx, opts)
		}
	}

	job, ok := objType.(*batchv1.Job)
	if ok {
		name = job.Name
		listF = func(ctx context.Context, opts metav1.ListOptions) (runtime.Object, error) {
			return clientSet.BatchV1().Jobs(job.Namespace).List(ctx, opts)
		}
		watchF = func(ctx context.Context, opts metav1.ListOptions) (watch.Interface, error) {
			return clientSet.BatchV1().Jobs(job.Namespace).Watch(ctx, opts)
		}
	}

	lw := cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.FieldSelector = fields.OneTermEqualSelector(
				"metadata.name",
				name,
			).String()
			// return clientSet.BatchV1().Jobs(namespace).List(ctx, options)
			return listF(ctx, options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.FieldSelector = fields.OneTermEqualSelector(
				"metadata.name",
				name,
			).String()
			return watchF(ctx, options)
		},
	}

	conditionF := func(event watch.Event) (bool, error) {
		log.Printf("number %d: %+v", number, event)
		switch event.Type {
		case watch.Deleted, watch.Error:
			return true, nil
		}
		switch t := event.Object.(type) {
		case *batchv1.Job:
			for _, condition := range t.Status.Conditions {
				switch condition.Type {
				case batchv1.JobComplete, batchv1.JobFailed:
					return true, nil
				}
			}
		case *v1.Pod:
			switch t.Status.Phase {
			case v1.PodSucceeded, v1.PodFailed:
				return true, nil
			}
			for _, s := range t.Status.InitContainerStatuses {
				if isWaitingWthErr(s) {
					return true, nil
				}
			}
			for _, s := range t.Status.ContainerStatuses {
				if isWaitingWthErr(s) {
					return true, nil
				}
			}
		}

		return false, nil
	}

	_, err := watchtools.UntilWithSync(ctx, &lw, &batchv1.Job{}, nil, conditionF)
	if err != nil {
		panic(err.Error())
	}
}

func isWaitingWthErr(container v1.ContainerStatus) bool {
	if container.State.Waiting != nil {
		switch container.State.Waiting.Reason {
		// https://github.com/kubernetes/kubernetes/blob/master/pkg/kubelet/images/types.go#L28
		case "ImagePullBackOff", "ImageInspectError", "ErrImagePull", "ErrImageNeverPull", "RegistryUnavailable", "InvalidImageName":
			return true
		}
	}
	return false
}
