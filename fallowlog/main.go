package main

import (
	"bufio"
	"context"
	"io"
	"log"
	"path/filepath"
	"tryk8sclient/util"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/util/homedir"
)

func main() {
	clientSet, err := util.GetClient(filepath.Join(homedir.HomeDir(), ".kube", "config"))
	if err != nil {
		panic(err.Error())
	}
	pod, err := clientSet.CoreV1().Pods("default").Get(context.TODO(), "postgresql-captain", metav1.GetOptions{})
	if err != nil {
		panic(err.Error())
	}

	stream, err := clientSet.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &v1.PodLogOptions{
		Container: pod.Spec.Containers[0].Name,
		Follow:    true,
	}).Stream(context.TODO())
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
		log.Println(msg)
		if bufRrr == io.EOF {
			break
		}
	}
}
