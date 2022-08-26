package rest

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	cacheObj "github.com/ttsubo2000/esi-terraform-worker/tools/cache"
	"github.com/ttsubo2000/esi-terraform-worker/types"
	"k8s.io/klog/v2"
)

var Providers []types.Provider

func returnAllProviders(w http.ResponseWriter, r *http.Request) {
	klog.Info("Endpoint Hit: returnAllProviders")
	json.NewEncoder(w).Encode(Providers)
}

func returnSingleProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["name"]

	for _, provider := range Providers {
		if provider.ObjectMeta.Name == key {
			json.NewEncoder(w).Encode(provider)
		}
	}
}

func createNewProvider(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var provider types.Provider
	json.Unmarshal(reqBody, &provider)
	clientState.Add(&provider)
	Providers = append(Providers, provider)

	json.NewEncoder(w).Encode(provider)
}

func deleteProvider(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var provider types.Provider
	json.Unmarshal(reqBody, &provider)
	clientState.Delete(&provider)

	vars := mux.Vars(r)
	name := vars["name"]

	for index, provider := range Providers {
		if provider.ObjectMeta.Name == name {
			Providers = append(Providers[:index], Providers[index+1:]...)
		}
	}
}
