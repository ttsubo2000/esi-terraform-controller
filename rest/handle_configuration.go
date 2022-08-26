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

func returnAllConfigurations(w http.ResponseWriter, r *http.Request) {
	klog.Info("Endpoint Hit: returnAllConfigurations")
	json.NewEncoder(w).Encode(Configurations)
}

func returnSingleConfiguration(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	key := vars["name"]

	for _, configuration := range Configurations {
		if configuration.ObjectMeta.Name == key {
			json.NewEncoder(w).Encode(configuration)
		}
	}
}

func createNewConfiguration(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var configuration types.Configuration
	json.Unmarshal(reqBody, &configuration)
	clientState.Add(&configuration)
	Configurations = append(Configurations, configuration)

	json.NewEncoder(w).Encode(configuration)
}

func deleteConfiguration(w http.ResponseWriter, r *http.Request, clientState cacheObj.Store) {
	reqBody, _ := ioutil.ReadAll(r.Body)
	var configuration types.Configuration
	json.Unmarshal(reqBody, &configuration)
	clientState.Delete(&configuration)

	vars := mux.Vars(r)
	name := vars["name"]

	for index, configuration := range Configurations {
		if configuration.ObjectMeta.Name == name {
			Configurations = append(Configurations[:index], Configurations[index+1:]...)
		}
	}
}
