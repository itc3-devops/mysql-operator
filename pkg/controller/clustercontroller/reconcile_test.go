/*
Copyright 2018 Pressinfra SRL

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

package clustercontroller

import (
	"context"
	"fmt"
	"time"

	kubeinformers "k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/tools/record"

	. "github.com/onsi/ginkgo"
	. "github.com/onsi/gomega"

	api "github.com/presslabs/mysql-operator/pkg/apis/mysql/v1alpha1"
	controllerpkg "github.com/presslabs/mysql-operator/pkg/controller"
	fakeMyClient "github.com/presslabs/mysql-operator/pkg/generated/clientset/versioned/fake"
	informers "github.com/presslabs/mysql-operator/pkg/generated/informers/externalversions"
	"github.com/presslabs/mysql-operator/pkg/util/options"
	tutil "github.com/presslabs/mysql-operator/pkg/util/test"
)

var _ = Describe("Test cluster reconciliation queue", func() {
	var (
		client     *fake.Clientset
		myClient   *fakeMyClient.Clientset
		rec        *record.FakeRecorder
		cluster    *api.MysqlCluster
		ctx        context.Context
		controller *Controller
		stop       chan struct{}
		opt        *options.Options
	)

	BeforeEach(func() {
		opt = options.GetOptions()
		client = fake.NewSimpleClientset()
		myClient = fakeMyClient.NewSimpleClientset()
		rec = record.NewFakeRecorder(100)
		ctx = context.TODO()
		cluster = tutil.NewFakeCluster("asd")
		stop = make(chan struct{})
		controller = newController(stop, client, myClient, rec)
		// for fast tests, else reconcileTime will be ~5s
		reconcileTime = 10 * time.Millisecond
	})

	AfterEach(func() {
		close(stop)
	})

	Describe("Reconcile a cluster", func() {
		Context("cluster not ready", func() {
			It("reconciliation should fail, orc not configured", func() {
				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				opt.OrchestratorUri = ""
				err = controller.Reconcile(ctx, cluster)
				Expect(err).ToNot(Succeed())

				// Expect to arrive element on reconciliation queue
				e, shutdown := controller.reconcileQueue.Get()
				Expect(shutdown).To(Equal(false))
				Expect(e).To(Equal(fmt.Sprintf("%s/asd", tutil.Namespace)))
			})
			It("reconciliation should succeed", func() {
				opt.OrchestratorUri = "/devnull"
				_, err := myClient.MysqlV1alpha1().MysqlClusters(tutil.Namespace).Create(cluster)
				Expect(err).To(Succeed())

				err = controller.Reconcile(ctx, cluster)
				Expect(err).To(Succeed())
			})
		})
	})

})

func newController(stop chan struct{}, client *fake.Clientset,
	myClient *fakeMyClient.Clientset,
	rec *record.FakeRecorder,
) *Controller {

	sharedInformerFactory := informers.NewSharedInformerFactory(
		myClient, time.Second)
	kubeSharedInformerFactory := kubeinformers.NewSharedInformerFactory(
		client, time.Second)

	sharedInformerFactory.Start(stop)
	kubeSharedInformerFactory.Start(stop)

	return New(&controllerpkg.Context{
		KubeClient: client,
		Client:     myClient,
		KubeSharedInformerFactory: kubeSharedInformerFactory,
		SharedInformerFactory:     sharedInformerFactory,
		Recorder:                  rec,
	})
}
