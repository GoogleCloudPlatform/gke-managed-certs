/*
Copyright 2020 Google LLC

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    https://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package queue provides helper functions for operating
// on a rate limited workqueue.
package queue

import (
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"k8s.io/klog"

	"github.com/GoogleCloudPlatform/gke-managed-certs/pkg/utils/types"
)

// Add adds obj to queue.
func Add(queue workqueue.RateLimitingInterface, obj interface{}) {
	key, err := cache.MetaNamespaceKeyFunc(obj)
	if err != nil {
		runtime.HandleError(err)
		return
	}

	klog.Infof("Enqueuing %T: %+v", obj, obj)
	queue.AddRateLimited(key)
}

// AddId adds resource identified by id to queue.
func AddId(queue workqueue.RateLimitingInterface, id types.Id) {
	key := id.Name
	if len(id.Namespace) > 0 {
		key = id.Namespace + "/" + id.Name
	}
	queue.AddRateLimited(key)
}
