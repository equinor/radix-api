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

// Code generated by client-gen. DO NOT EDIT.

package v1

import (
	v1 "github.com/statoil/radix-operator/pkg/apis/radix/v1"
	scheme "github.com/statoil/radix-operator/pkg/client/clientset/versioned/scheme"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	watch "k8s.io/apimachinery/pkg/watch"
	rest "k8s.io/client-go/rest"
)

// RadixRegistrationsGetter has a method to return a RadixRegistrationInterface.
// A group's client should implement this interface.
type RadixRegistrationsGetter interface {
	RadixRegistrations(namespace string) RadixRegistrationInterface
}

// RadixRegistrationInterface has methods to work with RadixRegistration resources.
type RadixRegistrationInterface interface {
	Create(*v1.RadixRegistration) (*v1.RadixRegistration, error)
	Update(*v1.RadixRegistration) (*v1.RadixRegistration, error)
	Delete(name string, options *meta_v1.DeleteOptions) error
	DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error
	Get(name string, options meta_v1.GetOptions) (*v1.RadixRegistration, error)
	List(opts meta_v1.ListOptions) (*v1.RadixRegistrationList, error)
	Watch(opts meta_v1.ListOptions) (watch.Interface, error)
	Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.RadixRegistration, err error)
	RadixRegistrationExpansion
}

// radixRegistrations implements RadixRegistrationInterface
type radixRegistrations struct {
	client rest.Interface
	ns     string
}

// newRadixRegistrations returns a RadixRegistrations
func newRadixRegistrations(c *RadixV1Client, namespace string) *radixRegistrations {
	return &radixRegistrations{
		client: c.RESTClient(),
		ns:     namespace,
	}
}

// Get takes name of the radixRegistration, and returns the corresponding radixRegistration object, and an error if there is any.
func (c *radixRegistrations) Get(name string, options meta_v1.GetOptions) (result *v1.RadixRegistration, err error) {
	result = &v1.RadixRegistration{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("radixregistrations").
		Name(name).
		VersionedParams(&options, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// List takes label and field selectors, and returns the list of RadixRegistrations that match those selectors.
func (c *radixRegistrations) List(opts meta_v1.ListOptions) (result *v1.RadixRegistrationList, err error) {
	result = &v1.RadixRegistrationList{}
	err = c.client.Get().
		Namespace(c.ns).
		Resource("radixregistrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Do().
		Into(result)
	return
}

// Watch returns a watch.Interface that watches the requested radixRegistrations.
func (c *radixRegistrations) Watch(opts meta_v1.ListOptions) (watch.Interface, error) {
	opts.Watch = true
	return c.client.Get().
		Namespace(c.ns).
		Resource("radixregistrations").
		VersionedParams(&opts, scheme.ParameterCodec).
		Watch()
}

// Create takes the representation of a radixRegistration and creates it.  Returns the server's representation of the radixRegistration, and an error, if there is any.
func (c *radixRegistrations) Create(radixRegistration *v1.RadixRegistration) (result *v1.RadixRegistration, err error) {
	result = &v1.RadixRegistration{}
	err = c.client.Post().
		Namespace(c.ns).
		Resource("radixregistrations").
		Body(radixRegistration).
		Do().
		Into(result)
	return
}

// Update takes the representation of a radixRegistration and updates it. Returns the server's representation of the radixRegistration, and an error, if there is any.
func (c *radixRegistrations) Update(radixRegistration *v1.RadixRegistration) (result *v1.RadixRegistration, err error) {
	result = &v1.RadixRegistration{}
	err = c.client.Put().
		Namespace(c.ns).
		Resource("radixregistrations").
		Name(radixRegistration.Name).
		Body(radixRegistration).
		Do().
		Into(result)
	return
}

// Delete takes name of the radixRegistration and deletes it. Returns an error if one occurs.
func (c *radixRegistrations) Delete(name string, options *meta_v1.DeleteOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("radixregistrations").
		Name(name).
		Body(options).
		Do().
		Error()
}

// DeleteCollection deletes a collection of objects.
func (c *radixRegistrations) DeleteCollection(options *meta_v1.DeleteOptions, listOptions meta_v1.ListOptions) error {
	return c.client.Delete().
		Namespace(c.ns).
		Resource("radixregistrations").
		VersionedParams(&listOptions, scheme.ParameterCodec).
		Body(options).
		Do().
		Error()
}

// Patch applies the patch and returns the patched radixRegistration.
func (c *radixRegistrations) Patch(name string, pt types.PatchType, data []byte, subresources ...string) (result *v1.RadixRegistration, err error) {
	result = &v1.RadixRegistration{}
	err = c.client.Patch(pt).
		Namespace(c.ns).
		Resource("radixregistrations").
		SubResource(subresources...).
		Name(name).
		Body(data).
		Do().
		Into(result)
	return
}
