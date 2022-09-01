package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	"k8s.io/klog/v2"
)

func returnAllSecrets(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: returnAllSecrets")
	var SecretsList []*types.Secret
	list := clientState.List()
	for _, obj := range list {
		switch obj.(type) {
		case *types.Secret:
			secret := obj.(*types.Secret)
			SecretsList = append(SecretsList, secret)
		}
	}
	json.NewEncoder(w).Encode(SecretsList)
}

func returnSingleSecret(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: returnSingleSecret")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	obj, _, err := clientState.GetByKey(fmt.Sprintf("Secret/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Secret Not Found\n")
		return
	}
	secret := obj.(*types.Secret)
	json.NewEncoder(w).Encode(secret)
}

func createNewSecret(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: createNewSecret")
	reqBody, _ := ioutil.ReadAll(r.Body)
	var secret types.Secret
	json.Unmarshal(reqBody, &secret)
	clientState.Add(&secret)

	json.NewEncoder(w).Encode(secret)
}

func updateSecret(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: updateSecret")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	_, _, err := clientState.GetByKey(fmt.Sprintf("Secret/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Secret Not Found\n")
		return
	} else {
		reqBody, _ := ioutil.ReadAll(r.Body)
		var secret types.Secret
		json.Unmarshal(reqBody, &secret)
		if name == secret.ObjectMeta.Name && namespace == secret.ObjectMeta.Namespace {
			clientState.Update(&secret, false)
			json.NewEncoder(w).Encode(secret)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Invalid Request Body\n")
		}
	}
}

func deleteSecret(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: deleteSecret")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	obj, _, err := clientState.GetByKey(fmt.Sprintf("Secret/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Secret Not Found\n")
		return
	} else {
		secret := obj.(*types.Secret)
		clientState.Delete(secret)
	}
}
