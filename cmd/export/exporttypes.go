// maybe this package isn't needed
package export

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type K8SObject struct {
	Obj metav1.ObjectMeta
}
