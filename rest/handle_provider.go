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

func returnAllProviders(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: returnAllProviders")
	var ProvidersList []*types.Provider
	list := clientState.List()
	for _, obj := range list {
		switch obj.(type) {
		case *types.Provider:
			provider := obj.(*types.Provider)
			ProvidersList = append(ProvidersList, provider)
		}
	}
	json.NewEncoder(w).Encode(ProvidersList)
}

func returnSingleProvider(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: returnSingleProvider")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	obj, _, err := clientState.GetByKey(fmt.Sprintf("Provider/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Provider Not Found\n")
		return
	}
	provider := obj.(*types.Provider)
	json.NewEncoder(w).Encode(provider)
}

func createNewProvider(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: createNewProvider")
	reqBody, _ := ioutil.ReadAll(r.Body)
	var provider types.Provider
	json.Unmarshal(reqBody, &provider)
	clientState.Add(&provider)

	json.NewEncoder(w).Encode(provider)
}

func updateProvider(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: updateProvider")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	_, _, err := clientState.GetByKey(fmt.Sprintf("Provider/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Provider Not Found\n")
		return
	} else {
		reqBody, _ := ioutil.ReadAll(r.Body)
		var provider types.Provider
		json.Unmarshal(reqBody, &provider)
		if name == provider.ObjectMeta.Name && namespace == provider.ObjectMeta.Namespace {
			clientState.Update(&provider)
			json.NewEncoder(w).Encode(provider)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Invalid Request Body\n")
		}
	}
}

func deleteProvider(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: deleteProvider")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	obj, _, err := clientState.GetByKey(fmt.Sprintf("Provider/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Provider Not Found\n")
		return
	} else {
		provider := obj.(*types.Provider)
		clientState.Delete(provider)
	}
}
