package pod

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/statoil/radix-api/api/utils"
	"github.com/statoil/radix-api/models"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"

	radixclient "github.com/statoil/radix-operator/pkg/client/clientset/versioned"
	"k8s.io/client-go/kubernetes"
)

const rootPath = "/container"

// GetRoutes List the supported routes of this handler
func GetRoutes() models.Routes {
	routes := models.Routes{
		models.Route{
			Path:        rootPath + "/pods",
			Method:      "GET",
			HandlerFunc: GetPods,
			WatcherFunc: GetPodStream,
		},
	}

	return routes
}

// GetSubscriptions Lists subscriptions this handler offers
func GetSubscriptions() models.Subscriptions {
	subscriptions := models.Subscriptions{
		models.Subscription{
			SubcribeCommand:    "pod_subscribe",
			UnsubscribeCommand: "pod_unsubscribe",
			DataType:           "pod",
			HandlerFunc:        GetPodStream,
		},
	}

	return subscriptions
}

// GetPodStream Lists new pods
func GetPodStream(client kubernetes.Interface, radixclient radixclient.Interface, arg string, data chan []byte, unsubscribe chan struct{}) {
	watchList := cache.NewFilteredListWatchFromClient(client.CoreV1().RESTClient(), "pods", corev1.NamespaceAll,
		func(options *metav1.ListOptions) {
		})
	_, controller := cache.NewInformer(
		watchList,
		&corev1.Pod{},
		time.Second*30,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				pod := obj.(*corev1.Pod)
				body, _ := json.Marshal(Pod{Name: pod.Name})
				data <- body
			},
		},
	)

	stop := make(chan struct{})
	go func() {
		<-unsubscribe
		close(stop)
	}()

	go controller.Run(stop)

}

// GetPods list pods
func GetPods(client kubernetes.Interface, radixclient radixclient.Interface, w http.ResponseWriter, r *http.Request) {
	// swagger:operation GET /container/pods pods getPods
	// ---
	// summary: Gets a list of all pods
	// responses:
	//   "200":
	//     description: "Successful operation"
	//     schema:
	//        type: "array"
	//        items:
	//           "$ref": "#/definitions/Pod"
	//   "401":
	//     description: "Unauthorized"
	//   "404":
	//     description: "Not found"
	pods, err := HandleGetPods(client)

	if err != nil {
		utils.ErrorResponse(w, r, err)
		return
	}

	utils.JSONResponse(w, r, &pods)
}
