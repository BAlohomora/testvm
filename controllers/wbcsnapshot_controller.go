/*
Copyright 2023.

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

package controllers

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	snapshotv1alpha1 "github.com/BAlohomora/testvm.git/api/v1alpha1"
)

// WbcSnapshotReconciler reconciles a WbcSnapshot object
type WbcSnapshotReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=snapshot.wbc.com,resources=wbcsnapshots,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=snapshot.wbc.com,resources=wbcsnapshots/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=snapshot.wbc.com,resources=wbcsnapshots/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the WbcSnapshot object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.14.1/pkg/reconcile
func (r *WbcSnapshotReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = r.log.WithValues("wbcsnapshot",req.NamespacedName) //accessed the namespace name of the resource

	// TODO(user): your logic here
	//Client get Function : Grab relevant WbcSnapShot resource
	snapshot := &snapshotv1alpha1.WbcSnapshot{}
	snapshotReadErr := r.Get(ctx,req.NamespacedName,snapshot)

	//Exit early if something went wrong parsing the WbcSnapshot object
	if snapshotReadErr != nil || len(snapshot.Name) == 0{
		r.log.Error(snapshotReadErr,"Error encountered reading snapshot")
		return ctrl.Result{},snapshotReadErr
	}

	snapshotCreator := &corev1.Pod{}
	snapshotCreatorReadErr := r.Get(
		ctx, types.NamespacedName{Name: "snapshot-creator",Namespace: req.Namespace},snapshotCreator,
	)

	//exit if wbcsnapshot creator is already running
	if snapshotCreatorReadErr == nil{
		r.log.Error(snapshotCreatorReadErr,"Snapshot creater already running")
		return ctrl.Result{},snapshotCreatorReadErr
	}


	newPvName := "wbc-snapshot-pv-" + strconv.FormatInt(time.Now().Unix(),10) + "-" + snapshot.Spec.SourceVolumeName
	newPvcname := "wbc-snapshot-pvc-" + strconv.FormatInt(time.Now().Unix(),10) + "-" + snapshot.Spec.SourceClaimName

	//create a new Persistent Volume
	newPersistentVol := corev1.persistentVolume{
		ObjectMeta: metav1.ObjectMeta{
				Name: 	   newPvtName,
				Namespace: req.Namespace,
		},
		Spec: corev1.PersistentVolumeSpec{
			    StorageClassName: "manual",
				AccessModes: 	  []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
				Capacity: corev1.ResourceList{
						corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),		
				},
				PersistentVolumeSource: corev1.PersistentVolumtSource{
					HostPath: &corev1.HostPathVolumeSource{
						Path: snapshot.Spec.HostPath,
					},
				},
		},
	}






	pvCreateErr := r.Create(ctx, &newPersistentVol)
	if pvCreateErr != nil {
		r.Log.Error(pvCreateErr,"Error encountered creating new pv")
		return ctrl.Result{},pvCreateErr
	}

	_= r.Log.WithValues("Created New Snapshot Persistent Volume",req.NamespacedName)

	manualStorageClass := "manual"

	//create new persisitent volume connected to new persistent volume
	newPersistentVolClaim := corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:          newPvcName,
			Namespace:     req.Namespace,
		},
		Spec: corev1.PersistentVolumeSpec{
			StorageClassName: &manualStorageClass,
			VolumeName:		  newPvName,
			AccessModes: 	  []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.ResourceRequiremnts{
				   Requests: corev1.ResourceList{
					      corev1.ResourceName(corev1.ResourceStorage): resource.MustParse("10Gi"),		
					},
			},
		},
	}

	pvcCreateErr := r.Create(ctx, &newPersistentVolClaim)
	if pvcCreateErr !- nil{
		r.Log.Error(pvcCreateErr,"Error encountered creating new pvc")
		return ctrl.Result{}, pvcCreateErr
	}

	_= r.Log.WithValues("Created New Snapshot Persistent Volume claim", req.NamespacedName)



	snapshotCreatorPod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{
			Name:          "snapshot-creator",
			Namespace:     req.Namespace,
		},
		Spec: corev1.PodSpec{
			RestartPolicy: "Never",
			Volumes: []corev1.Volume{
				    {
						Name: "wbc-snapshot-" + snapshot.Spec.SourceClaimName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimSource{
								ClaimName: snapshot.Spec.SourceClaimName,
							},
						},
					},
					{
						Name: "wbc-snapshot-" + newPvcName,
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimSource{
								ClaimName: newPvcName,
							},
						},
					},
				},
				Container: []corev1.Container{
					{
						Name: "busybox",
						Image: "k8s.grc.io/busybox",
						Command: []string{
							    "/bin/sh",
								"-c",
								"cp /tmp/source/test.file /tmp/dest/test.file"},
						VolumeMounts: []corev1.VolumeMount{
							  {
								Name: "wbc-snapshot-" + snapshot.Spec.SourceClaimName,
								MountPath: "/tmp/source",
							  },
							  {
								Name: "wbc-snapshot-" + newpvcName,
								MountPath: "/tmp/dest",
							  },
						},
					},
				},
		},
	}

	podCreateErr := r.Create(ctx, &snapshotCreatorPod)
	if podCreateErr !- nil{
		r.Log.Error(pvCreateErr,"Error encountered creating snapshot pod")
		return ctrl.Result{}, podCreateErr
	}

	_= r.Log.WithValues("Instantiating snapshot-creator pod", req.NamespacedName)

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
//tell the manager to watch certain resources(watch wbc snapshot resources)
func (r *WbcSnapshotReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&snapshotv1alpha1.WbcSnapshot{}).
		Complete(r)
}
