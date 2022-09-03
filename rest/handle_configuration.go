package rest

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	"github.com/gorilla/mux"
	cacheObj "github.com/ttsubo2000/terraform-controller/tools/cache"
	"github.com/ttsubo2000/terraform-controller/types"
	"k8s.io/klog/v2"
)

func returnAllConfigurations(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: returnAllConfigurations")
	var ConfigurationsList []*types.Configuration
	list := clientState.List()
	for _, obj := range list {
		switch obj.(type) {
		case *types.Configuration:
			configuration := obj.(*types.Configuration)
			ConfigurationsList = append(ConfigurationsList, configuration)
		}
	}
	json.NewEncoder(w).Encode(ConfigurationsList)
}

func returnSingleConfiguration(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: returnSingleConfiguration")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	obj, _, err := clientState.GetByKey(fmt.Sprintf("Configuration/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Configuration Not Found\n")
		return
	}
	configuration := obj.(*types.Configuration)
	json.NewEncoder(w).Encode(configuration)
}

func createNewConfiguration(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: createNewConfiguration")
	reqBody, _ := ioutil.ReadAll(r.Body)
	var configuration types.Configuration
	json.Unmarshal(reqBody, &configuration)
	clientState.Add(&configuration)

	json.NewEncoder(w).Encode(configuration)
}

func updateConfiguration(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: updateConfiguration")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	_, _, err := clientState.GetByKey(fmt.Sprintf("Configuration/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Configuration Not Found\n")
		return
	} else {
		reqBody, _ := ioutil.ReadAll(r.Body)
		var configuration types.Configuration
		json.Unmarshal(reqBody, &configuration)
		if name == configuration.ObjectMeta.Name && namespace == configuration.ObjectMeta.Namespace {
			clientState.Update(&configuration, true)
			json.NewEncoder(w).Encode(configuration)
		} else {
			w.WriteHeader(http.StatusBadRequest)
			fmt.Fprint(w, "Invalid Request Body\n")
		}
	}
}

func deleteConfiguration(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	klog.Info("Endpoint Hit: deleteConfiguration")
	vars := mux.Vars(r)
	name := vars["name"]
	namespace := vars["namespace"]
	obj, _, err := clientState.GetByKey(fmt.Sprintf("Configuration/%s/%s", namespace, name))
	if err != nil {
		w.WriteHeader(http.StatusNotFound)
		fmt.Fprint(w, "Configuration Not Found\n")
		return
	} else {
		configuration := obj.(*types.Configuration)
		clientState.Delete(configuration)
	}
}
