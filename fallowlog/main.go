package main

import (
	"bufio"
	"context"
	"flag"
	"io"
	"log"
	"path/filepath"
	"time"

	"github.com/ray31245/tryk8sclient/util"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/util/homedir"
)

func main() {
	var namespace *string
	var podName *string
	var number *int
	var kubeconfig *string

	namespace = flag.String("namespace", "default", "choose namespace to select")
	podName = flag.String("podName", "", "choose pod")
	number = flag.Int("number", 0, "numbers of repeat listen")
	kubeconfig = flag.String("kubeconfig", filepath.Join(homedir.HomeDir(), ".kube", "config"), "<HomeDir>/kube/config")
	flag.Parse()

	clientSet, err := util.GetClient(*kubeconfig)
	if err != nil {
		panic(err.Error())
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	pod, err := clientSet.CoreV1().Pods(*namespace).Get(ctx, *podName, metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	for i := 1; i <= *number; i++ {
		go fetchPodLog(ctx, clientSet, pod, i)
	}
	for {
		time.Sleep(time.Second * 10)
	}
}

func fetchPodLog(ctx context.Context, clientSet *kubernetes.Clientset, pod *v1.Pod, number int) {
	stream, err := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		Container: pod.Spec.Containers[0].Name,
		Follow:    true,
	}).Stream(ctx)
	if err != nil {
		panic(err.Error())
	}
	defer stream.Close()

	if stream == nil {
		return
	}

	buf := *bufio.NewReader(stream)
	var bufRrr error
	msg := ""
	for {
		// TO DO: confirm when buf.ReadString cause error
		msg, bufRrr = buf.ReadString(byte('\n'))
		if bufRrr != nil && bufRrr != io.EOF {
			log.Println("GetLogs.Stream read into buffer: ", bufRrr)
			continue
		}
		log.Printf("number %v: %s", number, msg)
		if bufRrr == io.EOF {
			break
		}
	}
}
