/*
Copyright The Kubernetes Authors.

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

// Code generated by informer-gen. DO NOT EDIT.

package v1

import (
	time "time"

	radix_v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	versioned "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	internalinterfaces "github.com/statoil/radix-operator/pkg/client/informers/externalversions/internalinterfaces"
	v1 "github.com/statoil/radix-operator/pkg/client/listers/radix/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	runtime "k8s.io/apimachinery/pkg/runtime"
	watch "k8s.io/apimachinery/pkg/watch"
	cache "k8s.io/client-go/tools/cache"
)

// RadixRegistrationInformer provides access to a shared informer and lister for
// RadixRegistrations.
type RadixRegistrationInformer interface {
	Informer() cache.SharedIndexInformer
	Lister() v1.RadixRegistrationLister
}

type radixRegistrationInformer struct {
	factory          internalinterfaces.SharedInformerFactory
	tweakListOptions internalinterfaces.TweakListOptionsFunc
	namespace        string
}

// NewRadixRegistrationInformer constructs a new informer for RadixRegistration type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewRadixRegistrationInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers) cache.SharedIndexInformer {
	return NewFilteredRadixRegistrationInformer(client, namespace, resyncPeriod, indexers, nil)
}

// NewFilteredRadixRegistrationInformer constructs a new informer for RadixRegistration type.
// Always prefer using an informer factory to get a shared informer instead of getting an independent
// one. This reduces memory footprint and number of connections to the server.
func NewFilteredRadixRegistrationInformer(client versioned.Interface, namespace string, resyncPeriod time.Duration, indexers cache.Indexers, tweakListOptions internalinterfaces.TweakListOptionsFunc) cache.SharedIndexInformer {
	return cache.NewSharedIndexInformer(
		&cache.ListWatch{
			ListFunc: func(options meta_v1.ListOptions) (runtime.Object, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RadixV1().RadixRegistrations(namespace).List(options)
			},
			WatchFunc: func(options meta_v1.ListOptions) (watch.Interface, error) {
				if tweakListOptions != nil {
					tweakListOptions(&options)
				}
				return client.RadixV1().RadixRegistrations(namespace).Watch(options)
			},
		},
		&radix_v1.RadixRegistration{},
		resyncPeriod,
		indexers,
	)
}

func (f *radixRegistrationInformer) defaultInformer(client versioned.Interface, resyncPeriod time.Duration) cache.SharedIndexInformer {
	return NewFilteredRadixRegistrationInformer(client, f.namespace, resyncPeriod, cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc}, f.tweakListOptions)
}

func (f *radixRegistrationInformer) Informer() cache.SharedIndexInformer {
	return f.factory.InformerFor(&radix_v1.RadixRegistration{}, f.defaultInformer)
}

func (f *radixRegistrationInformer) Lister() v1.RadixRegistrationLister {
	return v1.NewRadixRegistrationLister(f.Informer().GetIndexer())
}
