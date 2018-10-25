package utils

import "k8s.io/client-go/tools/cache"

func StreamInformers(data chan []byte, unsubscribe chan struct{}, informers ...cache.SharedIndexInformer) {
	stop := make(chan struct{})
	go func() {
		<-unsubscribe
		close(stop)
	}()

	for _, informer := range informers {
		go informer.Run(stop)
	}
}
